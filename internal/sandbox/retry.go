package sandbox

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

func (d *RetryDecorator) CreateSandbox(ctx context.Context, image string, cmd []string) (SandboxID, error) {
	var id SandboxID
	var err error
	fn := func() error {
		id, err = d.manager.CreateSandbox(ctx, image, cmd)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return "", err
	}
	return id, nil
}

func (d *RetryDecorator) StartSandbox(ctx context.Context, id SandboxID) error {
	fn := func() error {
		return d.manager.StartSandbox(ctx, id)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) AttachToSandbox(ctx context.Context, id SandboxID) (io.Reader, io.WriteCloser, error) {
	var reader io.Reader
	var writer io.WriteCloser
	var err error
	fn := func() error {
		reader, writer, err = d.manager.AttachToSandbox(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return nil, nil, err
	}
	return reader, writer, nil
}

func (d *RetryDecorator) RemoveSandbox(ctx context.Context, id SandboxID) error {
	fn := func() error {
		return d.manager.RemoveSandbox(ctx, id)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) CopyFileToSandbox(ctx context.Context, id SandboxID, path string, mode int64, data []byte) error {
	fn := func() error {
		return d.manager.CopyFileToSandbox(ctx, id, path, mode, data)
	}
	return d.retry(ctx, fn)
}

func (d *RetryDecorator) LoadFileFromSandbox(ctx context.Context, id SandboxID, path string) ([]byte, error) {
	var data []byte
	var err error
	fn := func() error {
		data, err = d.manager.LoadFileFromSandbox(ctx, id, path)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return nil, err
	}
	return data, nil
}

func (d *RetryDecorator) WaitSandbox(ctx context.Context, id SandboxID) (StatusCode, error) {
	var code StatusCode
	var err error
	fn := func() error {
		code, err = d.manager.WaitSandbox(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return -1, err
	}
	return code, nil
}

func (d *RetryDecorator) ReadLogsFromSandbox(ctx context.Context, id SandboxID) (string, error) {
	var logs string
	var err error
	fn := func() error {
		logs, err = d.manager.ReadLogsFromSandbox(ctx, id)
		return err
	}
	if err := d.retry(ctx, fn); err != nil {
		return "", err
	}
	return logs, nil
}
