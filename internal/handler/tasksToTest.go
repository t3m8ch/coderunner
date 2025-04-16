package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/t3m8ch/coderunner/internal/containerctl"
	"github.com/t3m8ch/coderunner/internal/filesctl"
	"github.com/t3m8ch/coderunner/internal/model"
)

func HandleTasksToTest(
	ctx context.Context,
	filesManager filesctl.Manager,
	containerManager containerctl.Manager,
	tasksToTest chan model.Task,
) {
	for task := range tasksToTest {
		handleTaskToTest(ctx, filesManager, containerManager, task)
	}
}

func handleTaskToTest(
	ctx context.Context,
	filesManager filesctl.Manager,
	containerManager containerctl.Manager,
	task model.Task,
) {
	fmt.Printf("Task to test: %+v\n", task)

	executable, err := filesManager.LoadFile(
		ctx,
		task.ExecutableLocation.BucketName,
		task.ExecutableLocation.ObjectName,
	)
	if err != nil {
		fmt.Printf("Error loading executable from MinIO: %v\n", err)
		return
	}
	fmt.Println("Executable loaded")

	testsData, err := filesManager.LoadFile(
		ctx,
		task.TestsLocation.BucketName,
		task.TestsLocation.ObjectName,
	)
	if err != nil {
		fmt.Printf("Error loading tests from MinIO: %v\n", err)
		return
	}
	fmt.Println("Tests loaded")

	tests, err := model.ParseTestsJSON(testsData)
	if err != nil {
		fmt.Printf("Error parsing tests JSON: %v\n", err)
		return
	}
	fmt.Println("Tests parsed")

	for i := range tests {
		fmt.Printf("Test #%d\n", i)

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

		err = containerManager.CopyFileToContainer(ctx, containerID, testingExecPath, 0700, executable)
		if err != nil {
			fmt.Printf("Error copying executable to container: %v\n", err)
			return
		}
		fmt.Println("Executable copied to container")

		err = containerManager.CopyFileToContainer(ctx, containerID, inputFilePath, 0644, []byte(tests[i].Stdin))
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

		output = strings.Trim(output, " ")
		output = strings.Trim(output, "\n")
		output = strings.Trim(output, "\t")

		tests[i].Stdout = strings.Trim(output, " ")
		tests[i].Stdout = strings.Trim(output, "\n")
		tests[i].Stdout = strings.Trim(output, "\t")

		if statusCode == 0 && output == tests[i].Stdout {
			fmt.Println("Test passed")
		} else {
			fmt.Println("Test failed")
			fmt.Printf("Expected: %s\n", tests[i].Stdout)
			fmt.Printf("Actual: %s\n", output)
			fmt.Printf("Expected bytes: %q\n", []byte(tests[i].Stdout))
			fmt.Printf("Actual bytes:   %q\n", []byte(output))
		}

		err = containerManager.RemoveContainer(ctx, containerID)
		if err != nil {
			fmt.Printf("Error container removing: %v\n", err)
		}
		fmt.Println("Container removed")
	}
}
