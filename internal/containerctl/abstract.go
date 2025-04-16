package containerctl

import (
	"context"
	"io"
)

type ContainerID = string
type StatusCode = int64

type Manager interface {
	CreateContainer(ctx context.Context, image string, cmd []string) (ContainerID, error)
	StartContainer(ctx context.Context, id ContainerID) error
	AttachToContainer(ctx context.Context, id ContainerID) (io.Reader, io.WriteCloser, error)
	RemoveContainer(ctx context.Context, id ContainerID) error
	CopyFileToContainer(ctx context.Context, id ContainerID, path string, mode int64, data []byte) error
	LoadFileFromContainer(ctx context.Context, id ContainerID, path string) ([]byte, error)
	WaitContainer(ctx context.Context, id ContainerID) (StatusCode, error)
	ReadLogsFromContainer(ctx context.Context, id ContainerID) (string, error)
}
