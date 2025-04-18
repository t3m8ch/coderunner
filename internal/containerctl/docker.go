package containerctl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type DockerManager struct {
	dockerClient *docker.Client
	cmd          []string
	execIDs      map[ContainerID]string
	execOutputs  map[ContainerID]string
	outputReady  map[ContainerID]chan struct{}
}

func NewDockerManager(dockerClient *docker.Client) *DockerManager {
	return &DockerManager{
		dockerClient: dockerClient,
		cmd:          make([]string, 0),
		execIDs:      make(map[ContainerID]string),
		execOutputs:  make(map[ContainerID]string),
		outputReady:  make(map[ContainerID]chan struct{}),
	}
}

func (m *DockerManager) CreateContainer(ctx context.Context, image string, cmd []string) (ContainerID, error) {
	m.cmd = cmd
	resp, err := m.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			OpenStdin:    true,
			StdinOnce:    true,
			Image:        image,
			Cmd:          []string{"tail", "-f", "/dev/null"},
		},
		&container.HostConfig{
			Tmpfs: map[string]string{
				"/app": "rw,exec,nosuid,size=65536k",
				"/tmp": "rw,exec,nosuid,size=65536k",
			},
			LogConfig: container.LogConfig{
				Type: "none",
			},
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", err
	}

	err = m.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (m *DockerManager) StartContainer(ctx context.Context, id ContainerID) error {
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          m.cmd,
	}
	execResp, err := m.dockerClient.ContainerExecCreate(ctx, string(id), execConfig)
	if err != nil {
		return err
	}

	m.execIDs[id] = execResp.ID
	m.outputReady[id] = make(chan struct{})

	attachResp, err := m.dockerClient.ContainerExecAttach(
		ctx,
		execResp.ID,
		container.ExecAttachOptions{},
	)
	if err != nil {
		return err
	}

	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, attachResp.Reader)
		if err != nil && err != io.EOF {
			// Логирование ошибки, если требуется
		}
		m.execOutputs[id] = buf.String()
		close(m.outputReady[id])
		attachResp.Close()
	}()

	return nil
}

func (m *DockerManager) AttachToContainer(ctx context.Context, id ContainerID) (io.Reader, io.WriteCloser, error) {
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

func (m *DockerManager) RemoveContainer(ctx context.Context, id ContainerID) error {
	return m.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (m *DockerManager) CopyFileToContainer(ctx context.Context, id ContainerID, path string, mode int64, data []byte) error {
	// Convert mode to octal string for chmod
	modeStr := fmt.Sprintf("%o", mode)

	// Create exec config
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: false,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"/bin/sh", "-c", "mkdir -p \"$(dirname \"$1\")\" && cat > \"$1\" && chmod $2 \"$1\"", "-", path, modeStr},
		// Cmd: []string{"ls"},
	}

	// Create exec instance
	execResp, err := m.dockerClient.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return err
	}
	fmt.Println("Container exec created")

	// Attach to exec instance with stdin
	attachResp, err := m.dockerClient.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{
		Detach: false,
	})
	if err != nil {
		return err
	}
	defer attachResp.Close()
	fmt.Println("Container exec attached")

	// Write the byte array to stdin
	_, err = io.Copy(attachResp.Conn, bytes.NewReader(data))
	if err != nil {
		return err
	}
	fmt.Println("Container exec data written")

	// Close stdin to signal EOF
	err = attachResp.Conn.Close()
	if err != nil {
		return err
	}
	fmt.Println("Container exec stdin closed")

	// Wait for exec to finish
	for {
		inspect, err := m.dockerClient.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return fmt.Errorf("failed to write file: exit code %d", inspect.ExitCode)
			}
			fmt.Println("Container exec finished")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *DockerManager) LoadFileFromContainer(ctx context.Context, id ContainerID, path string) ([]byte, error) {
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"cat", path},
	}
	execResp, err := m.dockerClient.ContainerExecCreate(ctx, string(id), execConfig)
	if err != nil {
		return nil, err
	}
	attachResp, err := m.dockerClient.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, err
	}
	defer attachResp.Close()
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil {
		return nil, err
	}
	inspectResp, err := m.dockerClient.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, err
	}
	if inspectResp.ExitCode != 0 {
		return nil, fmt.Errorf("failed to read file: exit code %d, stderr: %s", inspectResp.ExitCode, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (m *DockerManager) WaitContainer(ctx context.Context, id ContainerID) (StatusCode, error) {
	execID, ok := m.execIDs[id]
	if !ok {
		return -1, fmt.Errorf("no exec ID found for container %s", id)
	}

	for {
		execInspect, err := m.dockerClient.ContainerExecInspect(ctx, execID)
		if err != nil {
			return -1, err
		}
		if !execInspect.Running {
			return int64(execInspect.ExitCode), nil
		}
		time.Sleep(100 * time.Millisecond) // Wait a short time before checking again
	}
}

func (m *DockerManager) ReadLogsFromContainer(ctx context.Context, id ContainerID) (string, error) {
	readyCh, ok := m.outputReady[id]
	if !ok {
		return "", fmt.Errorf("no output ready for container %s", id)
	}
	select {
	case <-readyCh:
		output, ok := m.execOutputs[id]
		if !ok {
			return "", fmt.Errorf("no output for container %s", id)
		}
		return output, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
