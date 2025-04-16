package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/model"
)

func HandleStartTaskCommands(
	ctx context.Context,
	redisClient *redis.Client,
	tasksToCompile chan model.Task,
) {
	pubsub := redisClient.Subscribe(ctx, taskChannel)
	for msg := range pubsub.Channel() {
		var taskCommand model.StartTaskCommand
		err := json.Unmarshal([]byte(msg.Payload), &taskCommand)
		if err != nil {
			fmt.Printf("Error unmarshaling task: %v\n", err)
			continue
		}

		fmt.Printf("Received task: %+v\n", taskCommand)

		task := model.Task{
			ID:            taskCommand.ID,
			CodeLocation:  taskCommand.CodeLocation,
			TestsLocation: taskCommand.TestsLocation,
			Compiler:      taskCommand.Compiler,
			State:         model.CompilingTaskState,
		}
		jsonBytes, err := json.Marshal(task)
		if err != nil {
			fmt.Printf("Error marshaling task: %v\n", err)
			continue
		}

		redisClient.Set(ctx, fmt.Sprintf("task:%s", taskCommand.ID), string(jsonBytes), 0)

		tasksToCompile <- task
	}
}
