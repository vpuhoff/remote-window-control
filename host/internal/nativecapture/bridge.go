package nativecapture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var ErrBridgeUnavailable = errors.New("native capture bridge unavailable")

const streamFrameHeaderSize = 24

type Bridge struct {
	probePath string
}

type StreamFrame struct {
	Width   int
	Height  int
	Stride  int
	FrameID int64
	Data    []byte
}

type StreamSession struct {
	command *exec.Cmd
	stdout  *bufio.Reader
	stderr  bytes.Buffer
	waitCh  chan error
	waitMu  sync.Mutex
	waitErr error
	waited  bool
}

func NewBridge(baseDir string) *Bridge {
	probePath := resolveProbePath(baseDir)
	return &Bridge{
		probePath: probePath,
	}
}

func resolveProbePath(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, "native-capture", "tests", "CaptureProbe", "bin", "Debug", "net6.0-windows10.0.19041.0", "CaptureProbe.exe"),
		filepath.Join(baseDir, "native-capture", "tests", "CaptureProbe", "bin", "Release", "net6.0-windows10.0.19041.0", "CaptureProbe.exe"),
		filepath.Join(baseDir, "native-capture", "tests", "CaptureProbe", "bin", "Debug", "net6.0-windows", "CaptureProbe.exe"),
		filepath.Join(baseDir, "native-capture", "tests", "CaptureProbe", "bin", "Release", "net6.0-windows", "CaptureProbe.exe"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return candidates[0]
}

func (b *Bridge) CaptureSnapshot(ctx context.Context, request SnapshotRequest) (SnapshotResult, error) {
	if _, err := os.Stat(b.probePath); err != nil {
		return SnapshotResult{}, ErrBridgeUnavailable
	}

	command := exec.CommandContext(
		ctx,
		b.probePath,
		"--hwnd",
		fmt.Sprintf("%d", request.Handle),
		"--out",
		request.OutputPath,
	)

	output, err := command.Output()
	if err != nil {
		return SnapshotResult{}, err
	}

	var result SnapshotResult
	if err := json.Unmarshal(output, &result); err != nil {
		return SnapshotResult{}, err
	}

	return result, nil
}

func (b *Bridge) ListWindows(ctx context.Context) ([]WindowInfo, error) {
	if _, err := os.Stat(b.probePath); err != nil {
		return nil, ErrBridgeUnavailable
	}

	command := exec.CommandContext(
		ctx,
		b.probePath,
		"--list",
	)

	output, err := command.Output()
	if err != nil {
		return nil, err
	}

	var result []WindowInfo
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (b *Bridge) CapturePNG(ctx context.Context, handle uint64) ([]byte, error) {
	if _, err := os.Stat(b.probePath); err != nil {
		return nil, ErrBridgeUnavailable
	}

	tempFile, err := os.CreateTemp("", "share-app-capture-*.png")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempPath)

	result, err := b.CaptureSnapshot(ctx, SnapshotRequest{
		Handle:     handle,
		OutputPath: tempPath,
	})
	if err != nil {
		return nil, err
	}

	if result.Path == "" {
		result.Path = tempPath
	}

	return os.ReadFile(result.Path)
}

func (b *Bridge) OpenStream(ctx context.Context, handle uint64) (*StreamSession, error) {
	if _, err := os.Stat(b.probePath); err != nil {
		return nil, ErrBridgeUnavailable
	}

	command := exec.CommandContext(
		ctx,
		b.probePath,
		"--stream",
		"--hwnd",
		fmt.Sprintf("%d", handle),
	)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	session := &StreamSession{
		command: command,
		stdout:  bufio.NewReader(stdout),
		waitCh:  make(chan error, 1),
	}
	command.Stderr = &session.stderr

	if err := command.Start(); err != nil {
		return nil, err
	}

	go func() {
		session.waitCh <- command.Wait()
	}()

	return session, nil
}

func (s *StreamSession) ReadFrame() (StreamFrame, error) {
	var header [streamFrameHeaderSize]byte
	if _, err := io.ReadFull(s.stdout, header[:]); err != nil {
		return StreamFrame{}, s.wrapError(err)
	}

	payloadLength := binary.LittleEndian.Uint32(header[0:4])
	width := int(int32(binary.LittleEndian.Uint32(header[4:8])))
	height := int(int32(binary.LittleEndian.Uint32(header[8:12])))
	stride := int(int32(binary.LittleEndian.Uint32(header[12:16])))
	frameID := int64(binary.LittleEndian.Uint64(header[16:24]))

	if payloadLength == 0 {
		return StreamFrame{}, fmt.Errorf("capture stream returned empty payload")
	}

	data := make([]byte, int(payloadLength))
	if _, err := io.ReadFull(s.stdout, data); err != nil {
		return StreamFrame{}, s.wrapError(err)
	}

	return StreamFrame{
		Width:   width,
		Height:  height,
		Stride:  stride,
		FrameID: frameID,
		Data:    data,
	}, nil
}

func (s *StreamSession) Close() error {
	if s == nil || s.command == nil {
		return nil
	}

	if s.command.Process != nil {
		_ = s.command.Process.Kill()
	}

	err := s.wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}

	return err
}

func (s *StreamSession) wrapError(err error) error {
	waitErr := err
	if commandErr, ok := s.tryWait(); ok && commandErr != nil {
		waitErr = commandErr
	}

	if stderr := s.stderr.String(); stderr != "" {
		return fmt.Errorf("%w: %s", waitErr, stderr)
	}

	return waitErr
}

func (s *StreamSession) wait() error {
	s.waitMu.Lock()
	if s.waited {
		err := s.waitErr
		s.waitMu.Unlock()
		return err
	}
	s.waitMu.Unlock()

	err := <-s.waitCh

	s.waitMu.Lock()
	if !s.waited {
		s.waitErr = err
		s.waited = true
	}
	err = s.waitErr
	s.waitMu.Unlock()

	return err
}

func (s *StreamSession) tryWait() (error, bool) {
	s.waitMu.Lock()
	if s.waited {
		err := s.waitErr
		s.waitMu.Unlock()
		return err, true
	}
	s.waitMu.Unlock()

	select {
	case err := <-s.waitCh:
		s.waitMu.Lock()
		if !s.waited {
			s.waitErr = err
			s.waited = true
		}
		err = s.waitErr
		s.waitMu.Unlock()
		return err, true
	default:
		return nil, false
	}
}
