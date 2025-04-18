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

	testsCh := make(chan model.Test, 20)
	testsResultsCh := make(chan model.TestResult, len(tests))

	go func() {
		for i := range tests {
			testsCh <- model.Test{
				ID:     i,
				Stdin:  tests[i].Stdin,
				Stdout: tests[i].Stdout,
			}
		}
	}()

	go func() {
		fmt.Println("Waiting...")
		wg.Wait()
		fmt.Println("Done! Closing testsResultsCh and testsCh")
		close(testsResultsCh)
		close(testsCh)
	}()

	for test := range testsCh {
		go func() {
			defer wg.Done()

			fmt.Printf("----- Test #%d ----- \n", test.ID)

			sandboxID, err := sandboxManager.CreateSandbox(
				ctx,
				"debian:bookworm",
				[]string{"sh", "-c", fmt.Sprintf("%s < %s", testingExecPath, inputFilePath)},
			)
			if err != nil {
				fmt.Printf("test #%d: Error creating sandbox: %v\n", test.ID, err)
				return
			}
			fmt.Printf("test #%d: Sandbox created\n", test.ID)

			err = sandboxManager.CopyFileToSandbox(ctx, sandboxID, testingExecPath, 0700, executable)
			if err != nil {
				fmt.Printf("test #%d: Error copying executable to sandbox: %v\n", test.ID, err)
				return
			}
			fmt.Printf("test #%d: Executable copied to sandbox\n", test.ID)

			err = sandboxManager.CopyFileToSandbox(ctx, sandboxID, inputFilePath, 0644, []byte(test.Stdin))
			if err != nil {
				fmt.Printf("test #%d: Error copying input data: %v\n", test.ID, err)
				return
			}

			err = sandboxManager.StartSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error starting sandbox: %v\n", test.ID, err)
				return
			}
			fmt.Printf("test #%d: Sandbox started\n", test.ID)

			statusCode, err := sandboxManager.WaitSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error waiting for sandbox: %v\n", test.ID, err)
				return
			}

			output, err := sandboxManager.ReadLogsFromSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error reading logs from sandbox: %v\n", test.ID, err)
				return
			}
			fmt.Printf("test #%d: Output read from sandbox\n", test.ID)
			fmt.Printf("test #%d: %s", test.ID, output)

			fmt.Printf("test #%d: Testing completed with exit code %d\n", test.ID, statusCode)

			output = strings.Trim(output, " ")
			output = strings.Trim(output, "\n")
			output = strings.Trim(output, "\t")

			test.Stdout = strings.Trim(output, " ")
			test.Stdout = strings.Trim(output, "\n")
			test.Stdout = strings.Trim(output, "\t")

			if statusCode == 0 && output == test.Stdout {
				fmt.Printf("test #%d: Test passed\n", test.ID)
				testsResultsCh <- model.TestResult{TaskID: task.ID, TestID: test.ID, Successful: true}
			} else {
				testsResultsCh <- model.TestResult{TaskID: task.ID, TestID: test.ID, Successful: false}
				fmt.Printf("test #%d: Test failed\n", test.ID)
				fmt.Printf("test #%d: Expected: %s\n", test.ID, test.Stdout)
				fmt.Printf("test #%d: Actual: %s\n", test.ID, output)
				fmt.Printf("test #%d: Expected bytes: %q\n", test.ID, []byte(test.Stdout))
				fmt.Printf("test #%d: Actual bytes:   %q\n", test.ID, []byte(output))
			}

			err = sandboxManager.RemoveSandbox(ctx, sandboxID)
			if err != nil {
				fmt.Printf("test #%d: Error sandbox removing: %v\n", test.ID, err)
			}
			fmt.Printf("test #%d: Sandbox removed\n", test.ID)
		}()
	}

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
