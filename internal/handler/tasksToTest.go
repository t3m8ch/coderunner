package handler

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/model"
)

const executablePath = "/app/exec.out"

func HandleTasksToTest(
	ctx context.Context,
	minioClient *minio.Client,
	dockerClient *client.Client,
	tasksToTest chan model.Task,
) {
	for task := range tasksToTest {
		handleTaskToTest(ctx, minioClient, dockerClient, task)
	}
}

func handleTaskToTest(
	ctx context.Context,
	minioClient *minio.Client,
	dockerClient *client.Client,
	task model.Task,
) {
	fmt.Printf("Task to test: %+v\n", task)

	executable, err := loadBinaryFromMinio(ctx, minioClient, &task.ExecutableLocation)
	if err != nil {
		fmt.Printf("Error loading executable from MinIO: %v\n", err)
		return
	}

	resp, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: "debian:bookworm",
			Cmd:   []string{executablePath},
		},
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		fmt.Printf("failed to create run container: %v\n", err)
		return
	}
	defer dockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

	err = copyExecutableToContainer(ctx, dockerClient, resp.ID, executable)
	if err != nil {
		fmt.Printf("Error copy exec to container: %v\n", err)
		return
	}

	if err = dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		fmt.Printf("failed to start container: %v\n", err)
		return
	}
	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		fmt.Printf("container wait error: %v", err)
		return
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, _ := getContainerLogs(ctx, dockerClient, resp.ID)
			fmt.Println(logs)
			fmt.Printf("process exited with code %d\n", status.StatusCode)
			return
		}
	}

	logs, err := getContainerLogs(ctx, dockerClient, resp.ID)
	if err != nil {
		fmt.Printf("failed to get logs: %v\n", err)
		return
	}

	fmt.Println(logs)
}

func loadBinaryFromMinio(
	ctx context.Context,
	minioClient *minio.Client,
	location *model.MinIOLocation,
) ([]byte, error) {
	obj, err := minioClient.GetObject(
		ctx,
		location.BucketName,
		location.ObjectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		fmt.Printf("Error getting object: %v\n", err)
		return nil, err
	}
	defer obj.Close()

	content, err := io.ReadAll(obj)
	if err != nil {
		fmt.Printf("Error reading object: %v\n", err)
		return nil, err
	}

	return content, nil
}

func copyExecutableToContainer(
	ctx context.Context,
	cli *client.Client,
	containerID string,
	data []byte,
) error {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	hdr := &tar.Header{
		Name: executablePath,
		Mode: 0700,
		Size: int64(len(data)),
		Uid:  1000,
		Gid:  1000,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	tw.Close()

	return cli.CopyToContainer(ctx, containerID, "/", buf, container.CopyToContainerOptions{})
}

func getContainerLogs(
	ctx context.Context,
	dockerClient *client.Client,
	containerID string,
) (string, error) {
	reader, err := dockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Details:    false,
		Follow:     false,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return "", err
	}

	return buf.String(), nil
}
