package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/t3m8ch/coderunner/internal/model"
)

func HandleReceivedTasks(
	ctx context.Context,
	minioClient *minio.Client,
	receivedTasks chan model.TaskState,
) {
	for task := range receivedTasks {
		fmt.Printf("Received task: %+v\n", task)

		obj, err := minioClient.GetObject(
			ctx,
			task.CodeLocation.BucketName,
			task.CodeLocation.ObjectName,
			minio.GetObjectOptions{},
		)
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
}
