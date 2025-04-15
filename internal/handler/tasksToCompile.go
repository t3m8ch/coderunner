package handler

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/model"
)

const (
	containerCodePath     = "/app/main.cpp"
	outputPath            = "/app/output"
	executablesBucketName = "executables"
)

func HandleTasksToCompile(
	ctx context.Context,
	minioClient *minio.Client,
	dockerClient *client.Client,
	tasksToCompile chan model.Task,
	tasksToTest chan model.Task,
) {
	for task := range tasksToCompile {
		handleTaskToCompile(ctx, minioClient, dockerClient, task, tasksToTest)
	}
}

func handleTaskToCompile(
	ctx context.Context,
	minioClient *minio.Client,
	dockerClient *client.Client,
	task model.Task,
	tasksToTest chan model.Task,
) {
	fmt.Printf("Task to compile: %+v\n", task)

	code, err := loadCodeFromMinio(ctx, minioClient, &task.CodeLocation)
	if err != nil {
		fmt.Printf("Error loading code from MinIO: %v\n", err)
		return
	}

	containerID, err := runContainer(ctx, dockerClient, "gcc:latest")
	if err != nil {
		fmt.Printf("Error running container: %v\n", err)
		return
	}

	defer func() {
		err = dockerClient.ContainerRemove(
			ctx,
			containerID,
			container.RemoveOptions{Force: true},
		)
		if err != nil {
			fmt.Printf("Error container removing: %v\n", err)
		}
	}()

	err = copyCodeToContainer(
		ctx,
		dockerClient,
		containerID,
		code,
	)
	if err != nil {
		fmt.Printf("Error copying code to container: %v\n", err)
		return
	}

	execResp, err := dockerClient.ContainerExecCreate(
		ctx,
		containerID,
		container.ExecOptions{
			Cmd:          []string{"g++", containerCodePath, "-o", outputPath, "-static"},
			AttachStderr: true,
			AttachStdout: true,
		},
	)
	if err != nil {
		fmt.Printf("Error creating exec: %v\n", err)
		return
	}

	attachResp, err := dockerClient.ContainerExecAttach(
		ctx,
		execResp.ID,
		container.ExecAttachOptions{},
	)
	if err != nil {
		fmt.Printf("Error attaching to exec: %v\n", err)
		return
	}
	defer attachResp.Close()

	for {
		inspectResp, err := dockerClient.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			fmt.Printf("Error inspecting exec: %v\n", err)
			continue
		}

		if !inspectResp.Running {
			if inspectResp.ExitCode != 0 {
				fmt.Printf("Compilation failed with exit code %d\n", inspectResp.ExitCode)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	executableReader, _, err := dockerClient.CopyFromContainer(ctx, containerID, outputPath)
	if err != nil {
		fmt.Printf("Error copying executable: %v\n", err)
		return
	}
	defer executableReader.Close()

	tarReader := tar.NewReader(executableReader)
	tarReader.Next()
	executableData, err := io.ReadAll(tarReader)
	if err != nil {
		fmt.Printf("Error reading executable: %v\n", err)
		return
	}

	objectName := fmt.Sprintf("%s.out", task.ID)

	_, err = minioClient.PutObject(
		ctx,
		executablesBucketName,
		objectName,
		bytes.NewReader(executableData),
		int64(len(executableData)),
		minio.PutObjectOptions{},
	)
	if err != nil {
		fmt.Printf("Error put object to minio: %v\n", err)
		return
	}

	task.State = model.TestingTaskState
	task.ExecutableLocation = model.MinIOLocation{
		BucketName: executablesBucketName,
		ObjectName: objectName,
	}
	tasksToTest <- task
}

func loadCodeFromMinio(
	ctx context.Context,
	minioClient *minio.Client,
	location *model.MinIOLocation,
) (string, error) {
	obj, err := minioClient.GetObject(
		ctx,
		location.BucketName,
		location.ObjectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		fmt.Printf("Error getting object: %v\n", err)
		return "", err
	}
	defer obj.Close()

	content, err := io.ReadAll(obj)
	if err != nil {
		fmt.Printf("Error reading object: %v\n", err)
		return "", err
	}

	return string(content), nil
}

func runContainer(
	ctx context.Context,
	dockerClient *client.Client,
	image string,
) (string, error) {
	resp, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
			Cmd:   []string{"tail", "-f", "/dev/null"},
		},
		nil,
		nil,
		nil,
		"",
	)

	if err != nil {
		return "", err
	}

	containerID := resp.ID

	if err := dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", err
	}

	return containerID, nil
}

func copyCodeToContainer(
	ctx context.Context,
	dockerClient *client.Client,
	containerID string,
	code string,
) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: containerCodePath,
		Mode: 0644,
		Size: int64(len(code)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(code)); err != nil {
		return err
	}
	tw.Close()

	return dockerClient.CopyToContainer(
		ctx,
		containerID,
		"/",
		&buf,
		container.CopyToContainerOptions{},
	)
}
