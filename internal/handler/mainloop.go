package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/model"
)

const TASK_CHANNEL = "coderunner_task_channel"

func HandleStartTaskCommands(
	ctx context.Context,
	redisClient *redis.Client,
	receivedTasks chan model.TaskState,
) {
	pubsub := redisClient.Subscribe(ctx, TASK_CHANNEL)
	for msg := range pubsub.Channel() {
		var taskCommand model.StartTaskCommand
		err := json.Unmarshal([]byte(msg.Payload), &taskCommand)
		if err != nil {
			fmt.Printf("Error unmarshaling task: %v\n", err)
			continue
		}

		fmt.Printf("Received task: %+v\n", taskCommand)

		task := model.TaskState{
			ID:            taskCommand.ID,
			CodeLocation:  taskCommand.CodeLocation,
			TestsLocation: taskCommand.TestsLocation,
			Compiler:      taskCommand.Compiler,
			State:         model.PENDING_TASK_STATE,
		}
		jsonBytes, err := json.Marshal(task)
		if err != nil {
			fmt.Printf("Error marshaling task: %v\n", err)
			continue
		}

		redisClient.Set(ctx, fmt.Sprintf("task:%s", taskCommand.ID), string(jsonBytes), 0)

		receivedTasks <- task
	}
}
