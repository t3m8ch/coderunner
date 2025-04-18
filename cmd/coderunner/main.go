package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/client"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/t3m8ch/coderunner/internal/filesctl"
	"github.com/t3m8ch/coderunner/internal/handler"
	"github.com/t3m8ch/coderunner/internal/model"
	"github.com/t3m8ch/coderunner/internal/sandbox"
)

func main() {
	ctx := context.Background()

	redisClient := getRedisClient()
	defer redisClient.Close()

	minioClient := getMinioClient()

	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}

	var sandboxManager sandbox.Manager
	if strings.ToLower(os.Getenv("USE_TMPFS")) == "true" {
		fmt.Println("Using tmpfs")
		sandboxManager = sandbox.NewTMPFSDockerManager(dockerClient)
	} else {
		sandboxManager = sandbox.NewDockerManager(dockerClient)
	}

	filesManager := filesctl.NewMinioManager(minioClient)

	tasksToCompile := make(chan model.Task, 30)
	tasksToTest := make(chan model.Task, 2)

	fmt.Println("RUN!")

	for range 5 {
		go handler.HandleTasksToCompile(
			ctx,
			filesManager,
			sandboxManager,
			tasksToCompile,
			tasksToTest,
		)
	}

	for range 3 {
		go handler.HandleTasksToTest(
			ctx,
			filesManager,
			sandboxManager,
			tasksToTest,
			redisClient,
		)
	}

	handler.HandleStartTaskCommands(ctx, redisClient, tasksToCompile)
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
