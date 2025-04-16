package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/containerctl"
	"github.com/t3m8ch/coderunner/internal/model"
)

const executablePath = "/app/exec.out"

func HandleTasksToTest(
	ctx context.Context,
	minioClient *minio.Client,
	containerManager containerctl.Manager,
	tasksToTest chan model.Task,
) {
	for task := range tasksToTest {
		handleTaskToTest(ctx, minioClient, containerManager, task)
	}
}

func handleTaskToTest(
	ctx context.Context,
	minioClient *minio.Client,
	containerManager containerctl.Manager,
	task model.Task,
) {
	fmt.Printf("Task to test: %+v\n", task)

	executable, err := loadBinaryFromMinio(ctx, minioClient, &task.ExecutableLocation)
	if err != nil {
		fmt.Printf("Error loading executable from MinIO: %v\n", err)
		return
	}

	containerID, err := containerManager.CreateContainer(
		ctx,
		"debian:bookworm",
		[]string{executablePath},
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

	err = containerManager.CopyFileToContainer(ctx, containerID, executablePath, 0700, executable)
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

	logs, err := containerManager.ReadLogsFromContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error reading logs from container: %v\n", err)
	}
	fmt.Println(logs)
	fmt.Printf("Testing completed with exit code %d\n", statusCode)
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
