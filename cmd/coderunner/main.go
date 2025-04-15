package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/handler"
	"github.com/t3m8ch/coderunner/internal/model"
)

func main() {
	ctx := context.Background()

	redisClient := getRedisClient()
	defer redisClient.Close()

	minioClient := getMinioClient()

	receivedTasks := make(chan model.TaskState, 100)

	fmt.Println("RUN!")

	for range 5 {
		go handler.HandleReceivedTasks(ctx, minioClient, receivedTasks)
	}

	handler.HandleStartTaskCommands(ctx, redisClient, receivedTasks)
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
