package containerctl

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

func (d *ConcurrencyLimitDecorator) CreateContainer(ctx context.Context, image string, cmd []string) (ContainerID, error) {
	if err := d.acquire(ctx); err != nil {
		return "", err
	}
	defer d.release()
	return d.manager.CreateContainer(ctx, image, cmd)
}

func (d *ConcurrencyLimitDecorator) StartContainer(ctx context.Context, id ContainerID) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.StartContainer(ctx, id)
}

func (d *ConcurrencyLimitDecorator) AttachToContainer(ctx context.Context, id ContainerID) (io.Reader, io.WriteCloser, error) {
	if err := d.acquire(ctx); err != nil {
		return nil, nil, err
	}
	defer d.release()
	return d.manager.AttachToContainer(ctx, id)
}

func (d *ConcurrencyLimitDecorator) RemoveContainer(ctx context.Context, id ContainerID) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.RemoveContainer(ctx, id)
}

func (d *ConcurrencyLimitDecorator) CopyFileToContainer(ctx context.Context, id ContainerID, path string, mode int64, data []byte) error {
	if err := d.acquire(ctx); err != nil {
		return err
	}
	defer d.release()
	return d.manager.CopyFileToContainer(ctx, id, path, mode, data)
}

func (d *ConcurrencyLimitDecorator) LoadFileFromContainer(ctx context.Context, id ContainerID, path string) ([]byte, error) {
	if err := d.acquire(ctx); err != nil {
		return nil, err
	}
	defer d.release()
	return d.manager.LoadFileFromContainer(ctx, id, path)
}

func (d *ConcurrencyLimitDecorator) WaitContainer(ctx context.Context, id ContainerID) (StatusCode, error) {
	if err := d.acquire(ctx); err != nil {
		return -1, err
	}
	defer d.release()
	return d.manager.WaitContainer(ctx, id)
}

func (d *ConcurrencyLimitDecorator) ReadLogsFromContainer(ctx context.Context, id ContainerID) (string, error) {
	if err := d.acquire(ctx); err != nil {
		return "", err
	}
	defer d.release()
	return d.manager.ReadLogsFromContainer(ctx, id)
}
