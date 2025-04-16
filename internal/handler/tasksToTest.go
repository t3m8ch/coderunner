package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/containerctl"
	"github.com/t3m8ch/coderunner/internal/model"
)

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
	fmt.Println("Executable loaded")

	containerID, err := containerManager.CreateContainer(
		ctx,
		"debian:bookworm",
		[]string{"sh", "-c", fmt.Sprintf("%s < %s", testingExecPath, inputFilePath)},
	)
	if err != nil {
		fmt.Printf("Error creating container: %v\n", err)
		return
	}
	fmt.Println("Container created")

	defer func() {
		err = containerManager.RemoveContainer(ctx, containerID)
		if err != nil {
			fmt.Printf("Error container removing: %v\n", err)
		}
		fmt.Println("Container removed")
	}()

	err = containerManager.CopyFileToContainer(ctx, containerID, testingExecPath, 0700, executable)
	if err != nil {
		fmt.Printf("Error copying executable to container: %v\n", err)
		return
	}
	fmt.Println("Executable copied to container")

	err = containerManager.CopyFileToContainer(ctx, containerID, inputFilePath, 0644, []byte("5\n10 15\n20 25 30"))
	if err != nil {
		fmt.Printf("Error copying input data: %v\n", err)
		return
	}

	err = containerManager.StartContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error starting container: %v\n", err)
		return
	}
	fmt.Println("Container started")

	statusCode, err := containerManager.WaitContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error waiting for container: %v\n", err)
		return
	}

	output, err := containerManager.ReadLogsFromContainer(ctx, containerID)
	if err != nil {
		fmt.Printf("Error reading logs from container: %v\n", err)
		return
	}
	fmt.Println("Output read from container")
	fmt.Println(output)

	fmt.Printf("Testing completed with exit code %d\n", statusCode)
}

func loadBinaryFromMinio(
	ctx context.Context,
	minioClient *minio.Client,
	location *model.FileLocation,
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
