package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/model"
)

const TASK_CHANNEL = "coderunner_task_channel"
const MINIO_CODE_BUCKET_NAME = "code"
const MINIO_TESTS_BUCKET_NAME = "tests"

func main() {
	ctx := context.Background()

	redisClient := getRedisClient()
	defer redisClient.Close()

	minioClient := getMinioClient()

	receivedTasks := make(chan model.TaskState, 100)

	fmt.Println("RUN!")

	for range 5 {
		go func() {
			for task := range receivedTasks {
				fmt.Printf("Received task: %+v\n", task)

				obj, err := minioClient.GetObject(ctx, MINIO_CODE_BUCKET_NAME, "code.cpp", minio.GetObjectOptions{})
				if err != nil {
					fmt.Printf("Error getting object: %v\n", err)
					continue
				}
				defer obj.Close()

				content, err := io.ReadAll(obj)
				if err != nil {
					fmt.Printf("Error reading object: %v\n", err)
					continue
				}

				fmt.Println(string(content))
			}
		}()
	}

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

func getRedisClient() *redis.Client {
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})

	return redisClient
}

func getMinioClient() *minio.Client {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL := false

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		panic(err)
	}

	return client
}
