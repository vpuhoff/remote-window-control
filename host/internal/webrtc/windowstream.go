package webrtc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"

	"share-app-host/internal/nativecapture"
	"share-app-host/internal/targetwindow"

	pion "github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const streamFPS = 10
const idlePollInterval = 250 * time.Millisecond

func attachWindowVideoTrack(ctx context.Context, pc *pion.PeerConnection, bridge *nativecapture.Bridge, targets *targetwindow.Manager) error {
	track, err := pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeVP8},
		"video",
		"share-app",
	)
	if err != nil {
		return err
	}

	if _, err := pc.AddTrack(track); err != nil {
		return err
	}

	go streamSelectedWindow(ctx, bridge, targets, track)
	return nil
}

func streamSelectedWindow(ctx context.Context, bridge *nativecapture.Bridge, targets *targetwindow.Manager, track *pion.TrackLocalStaticSample) {
	var (
		currentHandle uint64
		stream        *nativecapture.StreamSession
		encoder       *vp8EncoderSession
	)
	defer func() {
		closeEncoderSession(encoder)
		closeStreamSession(stream)
	}()

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		handle, ok := targets.CurrentHandle()
		if !ok || handle == 0 {
			currentHandle = 0
			closeEncoderSession(encoder)
			encoder = nil
			closeStreamSession(stream)
			stream = nil
			sleepContext(ctx, idlePollInterval)
			continue
		}

		if stream == nil || handle != currentHandle {
			closeEncoderSession(encoder)
			encoder = nil
			closeStreamSession(stream)

			var err error
			stream, err = bridge.OpenStream(ctx, handle)
			if err != nil {
				log.Printf("window stream open error: %v", err)
				sleepContext(ctx, idlePollInterval)
				continue
			}
			currentHandle = handle
		}

		frame, err := stream.ReadFrame()
		if err != nil {
			log.Printf("window stream read error: %v", err)
			closeEncoderSession(encoder)
			encoder = nil
			closeStreamSession(stream)
			stream = nil
			sleepContext(ctx, idlePollInterval)
			continue
		}

		if encoder == nil || !encoder.matches(frame.Width, frame.Height) {
			closeEncoderSession(encoder)
			encoder, err = newVP8EncoderSession(ctx, track, frame.Width, frame.Height)
			if err != nil {
				log.Printf("window encoder start error: %v", err)
				encoder = nil
				sleepContext(ctx, idlePollInterval)
				continue
			}
		}

		if err := encoder.WriteFrame(frame.Data); err != nil {
			log.Printf("window stream encode error: %v", err)
			closeEncoderSession(encoder)
			encoder = nil
			sleepContext(ctx, 100*time.Millisecond)
		}
	}
}

type vp8EncoderSession struct {
	width      int
	height     int
	command    *exec.Cmd
	stdin      io.WriteCloser
	waitCh     chan error
	consumeCh  chan error
	waitMu     sync.Mutex
	waitErr    error
	waited     bool
	consumeMu  sync.Mutex
	consumeErr error
	consumed   bool
	stderr     bytes.Buffer
}

func newVP8EncoderSession(ctx context.Context, track *pion.TrackLocalStaticSample, width, height int) (*vp8EncoderSession, error) {
	size := fmt.Sprintf("%dx%d", width, height)
	command := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-video_size", size,
		"-framerate", fmt.Sprintf("%d", streamFPS),
		"-i", "pipe:0",
		"-an",
		"-vf", "format=yuv420p",
		"-c:v", "libvpx",
		"-b:v", "6M",
		"-crf", "10",
		"-deadline", "realtime",
		"-cpu-used", "4",
		"-auto-alt-ref", "0",
		"-g", fmt.Sprintf("%d", streamFPS*2),
		"-f", "ivf",
		"pipe:1",
	)

	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	session := &vp8EncoderSession{
		width:     width,
		height:    height,
		command:   command,
		stdin:     stdin,
		waitCh:    make(chan error, 1),
		consumeCh: make(chan error, 1),
	}
	command.Stderr = &session.stderr
	if err := command.Start(); err != nil {
		return nil, err
	}

	go func() {
		session.waitCh <- command.Wait()
	}()

	go func() {
		session.consumeCh <- consumeIVF(track, stdout)
	}()

	return session, nil
}

func consumeIVF(track *pion.TrackLocalStaticSample, stream io.Reader) error {
	reader := bufio.NewReader(stream)
	ivf, header, err := ivfreader.NewWith(reader)
	if err != nil {
		return err
	}

	frameDuration := time.Second
	if header.TimebaseDenominator != 0 {
		frameDuration = time.Second * time.Duration(header.TimebaseNumerator) / time.Duration(header.TimebaseDenominator)
	}

	for {
		payload, _, err := ivf.ParseNextFrame()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := track.WriteSample(media.Sample{
			Data:     payload,
			Duration: frameDuration,
		}); err != nil {
			return err
		}
	}
}

func (s *vp8EncoderSession) matches(width, height int) bool {
	return s != nil && s.width == width && s.height == height
}

func (s *vp8EncoderSession) WriteFrame(frame []byte) error {
	if s == nil {
		return fmt.Errorf("encoder session is nil")
	}

	if err, ok := s.tryConsume(); ok && err != nil && err != io.EOF {
		return err
	}

	if _, err := io.Copy(s.stdin, bytes.NewReader(frame)); err != nil {
		if stderr := s.stderr.String(); stderr != "" {
			return fmt.Errorf("%w: %s", err, stderr)
		}
		return err
	}

	return nil
}

func (s *vp8EncoderSession) Close() error {
	if s == nil {
		return nil
	}

	if s.stdin != nil {
		_ = s.stdin.Close()
	}

	waitErr := s.wait()
	consumeErr := s.consume()
	if consumeErr != nil && consumeErr != io.EOF {
		waitErr = consumeErr
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		waitErr = nil
	}

	if waitErr != nil && s.stderr.String() != "" {
		return fmt.Errorf("%w: %s", waitErr, s.stderr.String())
	}

	return waitErr
}

func (s *vp8EncoderSession) wait() error {
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

func (s *vp8EncoderSession) consume() error {
	s.consumeMu.Lock()
	if s.consumed {
		err := s.consumeErr
		s.consumeMu.Unlock()
		return err
	}
	s.consumeMu.Unlock()

	err := <-s.consumeCh

	s.consumeMu.Lock()
	if !s.consumed {
		s.consumeErr = err
		s.consumed = true
	}
	err = s.consumeErr
	s.consumeMu.Unlock()

	return err
}

func (s *vp8EncoderSession) tryConsume() (error, bool) {
	s.consumeMu.Lock()
	if s.consumed {
		err := s.consumeErr
		s.consumeMu.Unlock()
		return err, true
	}
	s.consumeMu.Unlock()

	select {
	case err := <-s.consumeCh:
		s.consumeMu.Lock()
		if !s.consumed {
			s.consumeErr = err
			s.consumed = true
		}
		err = s.consumeErr
		s.consumeMu.Unlock()
		return err, true
	default:
		return nil, false
	}
}

func closeStreamSession(stream *nativecapture.StreamSession) {
	if stream == nil {
		return
	}

	if err := stream.Close(); err != nil {
		log.Printf("window stream close error: %v", err)
	}
}

func closeEncoderSession(encoder *vp8EncoderSession) {
	if encoder == nil {
		return
	}

	if err := encoder.Close(); err != nil {
		log.Printf("window encoder close error: %v", err)
	}
}

func sleepContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
