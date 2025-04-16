package filesctl

import (
	"context"
)

type Manager interface {
	PutFile(ctx context.Context, bucket string, name string, data []byte) error
	LoadFile(ctx context.Context, bucket string, name string) ([]byte, error)
}
