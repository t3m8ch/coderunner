package sandbox

import (
	"context"
	"io"
)

type ConcurrencyLimitDecorator struct {
	manager   Manager
	semaphore chan struct{}
}

func NewConcurrencyLimitDecorator(manager Manager, maxConcurrent int) Manager {
	return &ConcurrencyLimitDecorator{
		manager:   manager,
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

func (d *ConcurrencyLimitDecorator) acquire(ctx context.Context) error {
	select {
	case d.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *ConcurrencyLimitDecorator) release() {
	<-d.semaphore
}

func (d *ConcurrencyLimitDecorator) CreateSandbox(ctx context.Context, image string, cmd []string) (SandboxID, error) {
	if err := d.acquire(ctx); err != nil {
		return "", err
	}
	defer d.release()
	return d.manager.CreateSandbox(ctx, image, cmd)
}

func (d *ConcurrencyLimitDecorator) StartSandbox(ctx context.Context, id SandboxID) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.StartSandbox(ctx, id)
}

func (d *ConcurrencyLimitDecorator) AttachToSandbox(ctx context.Context, id SandboxID) (io.Reader, io.WriteCloser, error) {
	if err := d.acquire(ctx); err != nil {
		return nil, nil, err
	}
	defer d.release()
	return d.manager.AttachToSandbox(ctx, id)
}

func (d *ConcurrencyLimitDecorator) RemoveSandbox(ctx context.Context, id SandboxID) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.RemoveSandbox(ctx, id)
}

func (d *ConcurrencyLimitDecorator) CopyFileToSandbox(ctx context.Context, id SandboxID, path string, mode int64, data []byte) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.CopyFileToSandbox(ctx, id, path, mode, data)
}

func (d *ConcurrencyLimitDecorator) LoadFileFromSandbox(ctx context.Context, id SandboxID, path string) ([]byte, error) {
	if err := d.acquire(ctx); err != nil {
		return nil, err
	}
	defer d.release()
	return d.manager.LoadFileFromSandbox(ctx, id, path)
}

func (d *ConcurrencyLimitDecorator) WaitSandbox(ctx context.Context, id SandboxID) (StatusCode, error) {
	if err := d.acquire(ctx); err != nil {
		return -1, err
	}
	defer d.release()
	return d.manager.WaitSandbox(ctx, id)
}

func (d *ConcurrencyLimitDecorator) ReadLogsFromSandbox(ctx context.Context, id SandboxID) (string, error) {
	if err := d.acquire(ctx); err != nil {
		return "", err
	}
	defer d.release()
	return d.manager.ReadLogsFromSandbox(ctx, id)
}
