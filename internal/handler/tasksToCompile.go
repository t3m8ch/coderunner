package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/containerctl"
	"github.com/t3m8ch/coderunner/internal/model"
)

func HandleTasksToCompile(
	ctx context.Context,
	minioClient *minio.Client,
	containerManager containerctl.Manager,
	tasksToCompile chan model.Task,
	tasksToTest chan model.Task,
) {
	for task := range tasksToCompile {
		handleTaskToCompile(ctx, minioClient, containerManager, task, tasksToTest)
	}
}

func handleTaskToCompile(
	ctx context.Context,
	minioClient *minio.Client,
	containerManager containerctl.Manager,
	task model.Task,
	tasksToTest chan model.Task,
) {
	fmt.Printf("Task to compile: %+v\n", task)

	code, err := loadCodeFromMinio(ctx, minioClient, &task.CodeLocation)
	if err != nil {
		fmt.Printf("Error loading code from MinIO: %v\n", err)
		return
	}

	containerID, err := containerManager.CreateContainer(
		ctx,
		"gcc:latest",
		[]string{"g++", sourceFilePath, "-o", compileExecPath, "-static"},
	)
	if err != nil {
		fmt.Printf("Error creating container: %v\n", err)
		return
	}

	defer func() {
		err = containerManager.RemoveContainer(ctx, containerID)
		if err != nil {
			fmt.Printf("Error container removing: %v\n", err)
		}
	}()

	err = containerManager.CopyFileToContainer(ctx, containerID, sourceFilePath, 0644, []byte(code))
	if err != nil {
		fmt.Printf("Error copying code to container: %v\n", err)
		return
	}

	err = containerManager.StartContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error starting container: %v\n", err)
		return
	}

	statusCode, err := containerManager.WaitContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error waiting for container: %v\n", err)
		return
	}
	if statusCode != 0 {
		fmt.Printf("Compilation failed with exit code %d\n", statusCode)
		logs, err := containerManager.ReadLogsFromContainer(ctx, containerID)
		if err != nil {
			fmt.Printf("Error reading logs from container: %v\n", err)
		}
		fmt.Println(logs)
		return
	}

	executable, err := containerManager.LoadFileFromContainer(ctx, containerID, compileExecPath)
	if err != nil {
		fmt.Printf("Error copying executable: %v\n", err)
		return
	}

	objectName := fmt.Sprintf("%s.out", task.ID)

	_, err = minioClient.PutObject(
		ctx,
		execBucketName,
		objectName,
		bytes.NewReader(executable),
		int64(len(executable)),
		minio.PutObjectOptions{},
	)
	if err != nil {
		fmt.Printf("Error put object to minio: %v\n", err)
		return
	}

	task.State = model.TestingTaskState
	task.ExecutableLocation = model.MinIOLocation{
		BucketName: execBucketName,
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
