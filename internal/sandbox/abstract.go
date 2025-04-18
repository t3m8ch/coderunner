package sandbox

import (
	"context"
	"io"
)

type SandboxID = string
type StatusCode = int64

type Manager interface {
	CreateSandbox(ctx context.Context, image string, cmd []string) (SandboxID, error)
	StartSandbox(ctx context.Context, id SandboxID) error
	AttachToSandbox(ctx context.Context, id SandboxID) (io.Reader, io.WriteCloser, error)
	RemoveSandbox(ctx context.Context, id SandboxID) error
	CopyFileToSandbox(ctx context.Context, id SandboxID, path string, mode int64, data []byte) error
	LoadFileFromSandbox(ctx context.Context, id SandboxID, path string) ([]byte, error)
	WaitSandbox(ctx context.Context, id SandboxID) (StatusCode, error)
	ReadLogsFromSandbox(ctx context.Context, id SandboxID) (string, error)
}
