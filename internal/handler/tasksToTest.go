package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/filesctl"
	"github.com/t3m8ch/coderunner/internal/model"
	"github.com/t3m8ch/coderunner/internal/sandbox"
)

func HandleTasksToTest(
	ctx context.Context,
	filesManager filesctl.Manager,
	sandboxManager sandbox.Manager,
	tasksToTest chan model.Task,
	redisClient *redis.Client,
) {
	for task := range tasksToTest {
		handleTaskToTest(ctx, filesManager, sandboxManager, redisClient, task)
	}
}

func handleTaskToTest(
	ctx context.Context,
	filesManager filesctl.Manager,
	sandboxManager sandbox.Manager,
	redisClient *redis.Client,
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

	testsResultsCh := make(chan model.TestResult, len(tests))

	for i := range tests {
		go func() {
			defer wg.Done()

			fmt.Printf("----- Test #%d ----- \n", i)

			sandboxID, err := sandboxManager.CreateSandbox(
				ctx,
				"debian:bookworm",
				[]string{"sh", "-c", fmt.Sprintf("%s < %s", testingExecPath, inputFilePath)},
			)
			if err != nil {
				fmt.Printf("test #%d: Error creating sandbox: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Sandbox created\n", i)

			err = sandboxManager.CopyFileToSandbox(ctx, sandboxID, testingExecPath, 0700, executable)
			if err != nil {
				fmt.Printf("test #%d: Error copying executable to sandbox: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Executable copied to sandbox\n", i)

			err = sandboxManager.CopyFileToSandbox(ctx, sandboxID, inputFilePath, 0644, []byte(tests[i].Stdin))
			if err != nil {
				fmt.Printf("test #%d: Error copying input data: %v\n", i, err)
				return
			}

			err = sandboxManager.StartSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error starting sandbox: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Sandbox started\n", i)

			statusCode, err := sandboxManager.WaitSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error waiting for sandbox: %v\n", i, err)
				return
			}

			output, err := sandboxManager.ReadLogsFromSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error reading logs from sandbox: %v\n", i, err)
				return
			}
			fmt.Printf("test #%d: Output read from sandbox\n", i)
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
				testsResultsCh <- model.TestResult{TaskID: task.ID, TestID: i, Successful: true}
			} else {
				testsResultsCh <- model.TestResult{TaskID: task.ID, TestID: i, Successful: false}
				fmt.Printf("test #%d: Test failed\n", i)
				fmt.Printf("test #%d: Expected: %s\n", i, tests[i].Stdout)
				fmt.Printf("test #%d: Actual: %s\n", i, output)
				fmt.Printf("test #%d: Expected bytes: %q\n", i, []byte(tests[i].Stdout))
				fmt.Printf("test #%d: Actual bytes:   %q\n", i, []byte(output))
			}

			err = sandboxManager.RemoveSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error sandbox removing: %v\n", i, err)
			}
			fmt.Printf("test #%d: Sandbox removed\n", i)
		}()
	}

	go func() {
		wg.Wait()
		close(testsResultsCh)
	}()

	task.TestsResults = make([]model.TestResult, 0, len(tests))
	for test := range testsResultsCh {
		task.TestsResults = append(task.TestsResults, test)
		jsonBytes, err := json.Marshal(test)
		if err != nil {
			fmt.Printf("test #%d: Error marshaling test result: %v\n", test.TestID, err)
		}
		redisClient.Publish(ctx, completedTestsChannel, string(jsonBytes))
	}

	fmt.Println("All tests completed!")
	fmt.Println(task.TestsResults)

	jsonBytes, err := json.Marshal(task)
	if err != nil {
		fmt.Printf("Error marshaling task: %v\n", err)
	}
	redisClient.Publish(ctx, completedTasksChannel, string(jsonBytes))
}
