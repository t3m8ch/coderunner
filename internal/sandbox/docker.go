package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
)

type DockerManager struct {
	dockerClient *docker.Client
}

func NewDockerManager(dockerClient *docker.Client) *DockerManager {
	return &DockerManager{dockerClient}
}

func (m *DockerManager) CreateSandbox(ctx context.Context, image string, cmd []string) (SandboxID, error) {
	resp, err := m.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			OpenStdin:    true,
			StdinOnce:    true,
			Image:        image,
			Cmd:          cmd,
		},
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (m *DockerManager) StartSandbox(ctx context.Context, id SandboxID) error {
	return m.dockerClient.ContainerStart(ctx, id, container.StartOptions{})
}

func (m *DockerManager) AttachToSandbox(ctx context.Context, id SandboxID) (io.Reader, io.WriteCloser, error) {
	resp, err := m.dockerClient.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, nil, err
	}
	return resp.Reader, resp.Conn, nil
}

func (m *DockerManager) RemoveSandbox(ctx context.Context, id SandboxID) error {
	return m.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (m *DockerManager) CopyFileToSandbox(ctx context.Context, id SandboxID, path string, mode int64, data []byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: path,
		Mode: mode,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	tw.Close()

	return m.dockerClient.CopyToContainer(
		ctx,
		id,
		"/",
		&buf,
		container.CopyToContainerOptions{},
	)
}

func (m *DockerManager) LoadFileFromSandbox(ctx context.Context, id SandboxID, path string) ([]byte, error) {
	reader, _, err := m.dockerClient.CopyFromContainer(ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	tarReader.Next()
	data, err := io.ReadAll(tarReader)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (m *DockerManager) WaitSandbox(ctx context.Context, id SandboxID) (StatusCode, error) {
	statusCh, errCh := m.dockerClient.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return -1, err
	case status := <-statusCh:
		return status.StatusCode, nil
	}
}

func (m *DockerManager) ReadLogsFromSandbox(ctx context.Context, id SandboxID) (string, error) {
	reader, err := m.dockerClient.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Details:    false,
		Follow:     false,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var buf bytes.Buffer

	// Это код, сгенерированный DeepSeek для очистки строки от всякого мусора.
	// Слава великой китайской абобе!

	// Полный ответ DeepSeek'а по ссылке: https://pastebin.com/UZQadXsf

	// Читаем логи с обработкой Docker-заголовков
	header := make([]byte, 8)
	for {
		// Читаем заголовок
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read header: %w", err)
		}

		// Разбираем размер данных (последние 4 байта заголовка, big-endian)
		dataSize := binary.BigEndian.Uint32(header[4:8])

		// Читаем данные
		data := make([]byte, dataSize)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			return "", fmt.Errorf("failed to read data: %w", err)
		}

		// Записываем данные в буфер
		buf.Write(data)
	}

	return buf.String(), nil
}
