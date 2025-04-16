package filesctl

import (
	"bytes"
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type MinioManager struct {
	client *minio.Client
}

func NewMinioManager(client *minio.Client) *MinioManager {
	return &MinioManager{client: client}
}

func (m *MinioManager) PutFile(ctx context.Context, bucket string, name string, data []byte) error {
	_, err := m.client.PutObject(ctx, bucket, name, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return err
}

func (m *MinioManager) LoadFile(ctx context.Context, bucket string, name string) ([]byte, error) {
	object, err := m.client.GetObject(ctx, bucket, name, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}

	return data, nil
}
