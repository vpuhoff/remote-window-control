package input

import (
	"errors"
	"time"
)

const (
	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0D
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkPrior  = 0x21
	vkNext   = 0x22
	vkEnd    = 0x23
	vkHome   = 0x24
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkDelete = 0x2E
)

var ErrTargetRequired = errors.New("target window is required for background input")

type SendInputInjector struct {
	targets TargetProvider
}

func NewSendInputInjector(targets TargetProvider) *SendInputInjector {
	return &SendInputInjector{targets: targets}
}

func (s *SendInputInjector) Move(x, y float64) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}
	clientX, clientY, ok := normalizePointClient(x, y, s.targets)
	if !ok {
		return ErrTargetRequired
	}
	return postMouseMove(s.targets, clientX, clientY)
}

func (s *SendInputInjector) Tap(button string, x, y float64) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}

	clientX, clientY, ok := normalizePointClient(x, y, s.targets)
	if !ok {
		return ErrTargetRequired
	}

	_ = postMouseMove(s.targets, clientX, clientY)
	if err := postMouseButton(s.targets, button, true, clientX, clientY); err != nil {
		return err
	}
	time.Sleep(25 * time.Millisecond)
	return postMouseButton(s.targets, button, false, clientX, clientY)
}

func (s *SendInputInjector) MouseDown(button string, x, y float64) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}
	clientX, clientY, ok := normalizePointClient(x, y, s.targets)
	if !ok {
		return ErrTargetRequired
	}
	_ = postMouseMove(s.targets, clientX, clientY)
	return postMouseButton(s.targets, button, true, clientX, clientY)
}

func (s *SendInputInjector) MouseUp(button string, x, y float64) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}
	clientX, clientY, ok := normalizePointClient(x, y, s.targets)
	if !ok {
		return ErrTargetRequired
	}
	return postMouseButton(s.targets, button, false, clientX, clientY)
}

func (s *SendInputInjector) Scroll(_, deltaY, x, y float64) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}
	clientX, clientY, ok := normalizePointClient(x, y, s.targets)
	if !ok {
		return ErrTargetRequired
	}
	return postMouseWheel(s.targets, int32(-deltaY*120), clientX, clientY)
}

func (s *SendInputInjector) ResizeViewport(command Command) error {
	scale := command.DevicePixelRatio
	if scale <= 0 {
		scale = 1
	}

	width := int(float64(command.Width) * scale)
	height := int(float64(command.Height) * scale)
	if width <= 0 || height <= 0 {
		return nil
	}

	return resizeTargetWindow(s.targets, width, height)
}

func (s *SendInputInjector) KeyDown(command Command) error {
	if vk, ok := specialKeyToVK(command.Key); ok {
		if s.targets == nil {
			return ErrTargetRequired
		}
		if h, ok := s.targets.CurrentHandle(); !ok || h == 0 {
			return ErrTargetRequired
		}
		return postKeyToWindow(s.targets, vk, false)
	}
	return nil
}

func (s *SendInputInjector) KeyUp(command Command) error {
	if vk, ok := specialKeyToVK(command.Key); ok {
		if s.targets == nil {
			return ErrTargetRequired
		}
		if h, ok := s.targets.CurrentHandle(); !ok || h == 0 {
			return ErrTargetRequired
		}
		return postKeyToWindow(s.targets, vk, true)
	}
	return nil
}

func (s *SendInputInjector) Text(text string) error {
	if s.targets == nil {
		return ErrTargetRequired
	}
	if handle, ok := s.targets.CurrentHandle(); !ok || handle == 0 {
		return ErrTargetRequired
	}
	return postText(s.targets, text)
}

func normalizePointClient(x, y float64, targets TargetProvider) (int32, int32, bool) {
	if targets == nil {
		return 0, 0, false
	}
	handle, ok := targets.CurrentHandle()
	if !ok || handle == 0 {
		return 0, 0, false
	}

	clientRect, ok := currentClientRect(handle)
	if !ok {
		return 0, 0, false
	}

	width := clientRect.Right - clientRect.Left
	height := clientRect.Bottom - clientRect.Top
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}

	return int32(float64(width) * clampUnit(x)), int32(float64(height) * clampUnit(y)), true
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func specialKeyToVK(key string) (uint16, bool) {
	switch key {
	case "Backspace":
		return vkBack, true
	case "Tab":
		return vkTab, true
	case "Enter":
		return vkReturn, true
	case "Escape":
		return vkEscape, true
	case "Delete":
		return vkDelete, true
	case "ArrowLeft":
		return vkLeft, true
	case "ArrowUp":
		return vkUp, true
	case "ArrowRight":
		return vkRight, true
	case "ArrowDown":
		return vkDown, true
	case "Home":
		return vkHome, true
	case "End":
		return vkEnd, true
	case "PageUp":
		return vkPrior, true
	case "PageDown":
		return vkNext, true
	case " ":
		return vkSpace, true
	default:
		return 0, false
	}
}
