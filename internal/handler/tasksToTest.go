package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"

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

	var wg sync.WaitGroup
	wg.Add(len(tests))

	for i := range tests {
		go func() {
			defer wg.Done()

			fmt.Printf("----- Test #%d ----- \n", i)

			containerID, err := containerManager.CreateContainer(
				ctx,
				"debian:bookworm",
				[]string{"sh", "-c", fmt.Sprintf("%s < %s", testingExecPath, inputFilePath)},
			)
			if err != nil {
				fmt.Printf("test #%d: Error creating container: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Container created\n", i)

			err = containerManager.CopyFileToContainer(ctx, containerID, testingExecPath, 0700, executable)
			if err != nil {
				fmt.Printf("test #%d: Error copying executable to container: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Executable copied to container\n", i)

			err = containerManager.CopyFileToContainer(ctx, containerID, inputFilePath, 0644, []byte(tests[i].Stdin))
			if err != nil {
				fmt.Printf("test #%d: Error copying input data: %v\n", i, err)
				return
			}

			err = containerManager.StartContainer(ctx, containerID)
			if err != nil {
				fmt.Printf("test #%d: Error starting container: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Container started\n", i)

			statusCode, err := containerManager.WaitContainer(ctx, containerID)
			if err != nil {
				fmt.Printf("test #%d: Error waiting for container: %v\n", i, err)
				return
			}

			output, err := containerManager.ReadLogsFromContainer(ctx, containerID)
			if err != nil {
				fmt.Printf("test #%d: Error reading logs from container: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Output read from container\n", i)
			fmt.Printf("test #%d: %s", i, output)

			fmt.Printf("test #%d: Testing completed with exit code %d\n", i, statusCode)

			output = strings.Trim(output, " ")
			output = strings.Trim(output, "\n")
			output = strings.Trim(output, "\t")

			tests[i].Stdout = strings.Trim(output, " ")
			tests[i].Stdout = strings.Trim(output, "\n")
			tests[i].Stdout = strings.Trim(output, "\t")

			if statusCode == 0 && output == tests[i].Stdout {
				fmt.Printf("test #%d: Test passed\n", i)
			} else {
				fmt.Printf("test #%d: Test failed\n", i)
				fmt.Printf("test #%d: Expected: %s\n", i, tests[i].Stdout)
				fmt.Printf("test #%d: Actual: %s\n", i, output)
				fmt.Printf("test #%d: Expected bytes: %q\n", i, []byte(tests[i].Stdout))
				fmt.Printf("test #%d: Actual bytes:   %q\n", i, []byte(output))
			}

			err = containerManager.RemoveContainer(ctx, containerID)
			if err != nil {
				fmt.Printf("test #%d: Error container removing: %v\n", i, err)
			}
			fmt.Printf("test #%d: Container removed\n", i)
		}()
	}

	wg.Wait()
	fmt.Println("All tests completed!")
}
