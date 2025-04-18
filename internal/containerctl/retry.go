package containerctl

import (
	"context"
	"fmt"
	"io"
	"time"
)

type RetryDecorator struct {
	manager Manager
	retries int
	delay   time.Duration
}

func NewRetryDecorator(manager Manager, retries int, delay time.Duration) Manager {
	return &RetryDecorator{
		manager: manager,
		retries: retries,
		delay:   delay,
	}
}

func (d *RetryDecorator) retry(ctx context.Context, fn func() error) error {
	var err error
	for i := 0; i < d.retries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.delay):
		}
	}
	return fmt.Errorf("after %d attempts: %w", d.retries, err)
}

func isRetryable(err error) bool {
	return err != context.Canceled && err != context.DeadlineExceeded
}

func (d *RetryDecorator) CreateContainer(ctx context.Context, image string, cmd []string) (ContainerID, error) {
	var id ContainerID
	var err error
	fn := func() error {
		id, err = d.manager.CreateContainer(ctx, image, cmd)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return "", err
	}
	return id, nil
}

func (d *RetryDecorator) StartContainer(ctx context.Context, id ContainerID) error {
	fn := func() error {
		return d.manager.StartContainer(ctx, id)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) AttachToContainer(ctx context.Context, id ContainerID) (io.Reader, io.WriteCloser, error) {
	var reader io.Reader
	var writer io.WriteCloser
	var err error
	fn := func() error {
		reader, writer, err = d.manager.AttachToContainer(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return nil, nil, err
	}
	return reader, writer, nil
}

func (d *RetryDecorator) RemoveContainer(ctx context.Context, id ContainerID) error {
	fn := func() error {
		return d.manager.RemoveContainer(ctx, id)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) CopyFileToContainer(ctx context.Context, id ContainerID, path string, mode int64, data []byte) error {
	fn := func() error {
		return d.manager.CopyFileToContainer(ctx, id, path, mode, data)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) LoadFileFromContainer(ctx context.Context, id ContainerID, path string) ([]byte, error) {
	var data []byte
	var err error
	fn := func() error {
		data, err = d.manager.LoadFileFromContainer(ctx, id, path)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return nil, err
	}
	return data, nil
}

func (d *RetryDecorator) WaitContainer(ctx context.Context, id ContainerID) (StatusCode, error) {
	var code StatusCode
	var err error
	fn := func() error {
		code, err = d.manager.WaitContainer(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return -1, err
	}
	return code, nil
}

func (d *RetryDecorator) ReadLogsFromContainer(ctx context.Context, id ContainerID) (string, error) {
	var logs string
	var err error
	fn := func() error {
		logs, err = d.manager.ReadLogsFromContainer(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return "", err
	}
	return logs, nil
}
