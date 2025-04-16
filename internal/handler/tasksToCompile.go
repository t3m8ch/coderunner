package handler

import (
	"context"
	"fmt"

	"github.com/t3m8ch/coderunner/internal/containerctl"
	"github.com/t3m8ch/coderunner/internal/filesctl"
	"github.com/t3m8ch/coderunner/internal/model"
)

func HandleTasksToCompile(
	ctx context.Context,
	filesManager filesctl.Manager,
	containerManager containerctl.Manager,
	tasksToCompile chan model.Task,
	tasksToTest chan model.Task,
) {
	for task := range tasksToCompile {
		handleTaskToCompile(ctx, filesManager, containerManager, task, tasksToTest)
	}
}

func handleTaskToCompile(
	ctx context.Context,
	filesManager filesctl.Manager,
	containerManager containerctl.Manager,
	task model.Task,
	tasksToTest chan model.Task,
) {
	fmt.Printf("Task to compile: %+v\n", task)

	codeBinary, err := filesManager.LoadFile(ctx, task.CodeLocation.BucketName, task.CodeLocation.ObjectName)
	if err != nil {
		fmt.Printf("Error loading code from file server: %v\n", err)
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

	err = containerManager.CopyFileToContainer(ctx, containerID, sourceFilePath, 0644, codeBinary)
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

	err = filesManager.PutFile(
		ctx,
		execBucketName,
		objectName,
		executable,
	)
	if err != nil {
		fmt.Printf("Error put object to file server: %v\n", err)
		return
	}

	task.State = model.TestingTaskState
	task.ExecutableLocation = model.FileLocation{
		BucketName: execBucketName,
		ObjectName: objectName,
	}
	tasksToTest <- task
}
