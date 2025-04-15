package main

import (
	"context"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const TASK_CHANNEL = "coderunner_task_channel"

func main() {
	ctx := context.Background()

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})

	defer redisClient.Close()

	pubsub := redisClient.Subscribe(ctx, TASK_CHANNEL)
	for msg := range pubsub.Channel() {
		println(msg.Payload)
	}
}
