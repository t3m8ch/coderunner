package handler

import (
	"context"
	"fmt"

	"github.com/t3m8ch/coderunner/internal/filesctl"
	"github.com/t3m8ch/coderunner/internal/model"
	"github.com/t3m8ch/coderunner/internal/sandbox"
)

func HandleTasksToCompile(
	ctx context.Context,
	filesManager filesctl.Manager,
	sandboxManager sandbox.Manager,
	tasksToCompile chan model.Task,
	tasksToTest chan model.Task,
) {
	for task := range tasksToCompile {
		handleTaskToCompile(ctx, filesManager, sandboxManager, task, tasksToTest)
	}
}

func handleTaskToCompile(
	ctx context.Context,
	filesManager filesctl.Manager,
	sandboxManager sandbox.Manager,
	task model.Task,
	tasksToTest chan model.Task,
) {
	fmt.Printf("Task to compile: %+v\n", task)

	codeBinary, err := filesManager.LoadFile(ctx, task.CodeLocation.BucketName, task.CodeLocation.ObjectName)
	if err != nil {
		fmt.Printf("Error loading code from file server: %v\n", err)
		return
	}

	sandboxID, err := sandboxManager.CreateSandbox(
		ctx,
		"gcc:latest",
		[]string{"g++", sourceFilePath, "-o", compileExecPath, "-static"},
	)
	if err != nil {
		fmt.Printf("Error creating sandbox: %v\n", err)
		return
	}

	defer func() {
		err = sandboxManager.RemoveSandbox(ctx, sandboxID)
		if err != nil {
			fmt.Printf("Error sandbox removing: %v\n", err)
		}
	}()

	err = sandboxManager.CopyFileToSandbox(ctx, sandboxID, sourceFilePath, 0644, codeBinary)
	if err != nil {
		fmt.Printf("Error copying code to sandbox: %v\n", err)
		return
	}

	err = sandboxManager.StartSandbox(ctx, sandboxID)
	if err != nil {
		fmt.Printf("Error starting sandbox: %v\n", err)
		return
	}

	statusCode, err := sandboxManager.WaitSandbox(ctx, sandboxID)
	if err != nil {
		fmt.Printf("Error waiting for sandbox: %v\n", err)
		return
	}
	if statusCode != 0 {
		fmt.Printf("Compilation failed with exit code %d\n", statusCode)
		logs, err := sandboxManager.ReadLogsFromSandbox(ctx, sandboxID)
		if err != nil {
			fmt.Printf("Error reading logs from sandbox: %v\n", err)
		}
		fmt.Println(logs)
		return
	}

	executable, err := sandboxManager.LoadFileFromSandbox(ctx, sandboxID, compileExecPath)
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
