package input

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	wmMouseWheel = 0x020A
)

var ErrTargetWindowNotSelected = errors.New("target window not selected")

type TargetProvider interface {
	CurrentHandle() (uint64, bool)
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

var (
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procMoveWindow          = user32.NewProc("MoveWindow")
)

func currentWindowRect(provider TargetProvider) (rect, bool) {
	if provider == nil {
		return rect{}, false
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return rect{}, false
	}

	var out rect
	result, _, _ := procGetWindowRect.Call(uintptr(handle), uintptr(unsafe.Pointer(&out)))
	if result == 0 {
		return rect{}, false
	}

	return out, true
}

func focusTargetWindow(provider TargetProvider) {
	if provider == nil {
		return
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return
	}

	_, _, _ = procSetForegroundWindow.Call(uintptr(handle))
}

func postMouseWheel(provider TargetProvider, delta int32) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	messageParam := uintptr(uint32(uint16(delta)) << 16)
	result, _, err := procPostMessageW.Call(
		uintptr(handle),
		uintptr(wmMouseWheel),
		messageParam,
		0,
	)
	if result == 0 && err != syscall.Errno(0) {
		return err
	}

	return nil
}

func resizeTargetWindow(provider TargetProvider, desiredClientWidth, desiredClientHeight int) error {
	if provider == nil {
		return ErrTargetWindowNotSelected
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return ErrTargetWindowNotSelected
	}

	windowRect, ok := currentWindowRect(provider)
	if !ok {
		return ErrTargetWindowNotSelected
	}

	clientRect, ok := currentClientRect(handle)
	if !ok {
		return ErrTargetWindowNotSelected
	}

	frameWidth := (windowRect.Right - windowRect.Left) - (clientRect.Right - clientRect.Left)
	frameHeight := (windowRect.Bottom - windowRect.Top) - (clientRect.Bottom - clientRect.Top)

	newWidth := clampDimension(desiredClientWidth + int(frameWidth))
	newHeight := clampDimension(desiredClientHeight + int(frameHeight))

	result, _, err := procMoveWindow.Call(
		uintptr(handle),
		uintptr(windowRect.Left),
		uintptr(windowRect.Top),
		uintptr(newWidth),
		uintptr(newHeight),
		1,
	)
	if result == 0 && err != syscall.Errno(0) {
		return err
	}

	return nil
}

func currentClientRect(handle uint64) (rect, bool) {
	var out rect
	result, _, _ := procGetClientRect.Call(uintptr(handle), uintptr(unsafe.Pointer(&out)))
	if result == 0 {
		return rect{}, false
	}

	return out, true
}

func clampDimension(value int) int {
	if value < 200 {
		return 200
	}
	if value > 4096 {
		return 4096
	}
	return value
}
