package input

import (
	"errors"
	"math"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	wmKeyDown                = 0x0100
	wmKeyUp                  = 0x0101
	wmChar                   = 0x0102
	wmMouseMove              = 0x0200
	wmLButtonDown            = 0x0201
	wmLButtonUp              = 0x0202
	wmRButtonDown            = 0x0204
	wmRButtonUp              = 0x0205
	wmMouseWheel             = 0x020A
	monitorDefaultToNearest = 0x00000002

	mkLButton = 0x0001
	mkRButton = 0x0002
)

var ErrTargetWindowNotSelected = errors.New("target window not selected")

var user32 = syscall.NewLazyDLL("user32.dll")

type TargetProvider interface {
	CurrentHandle() (uint64, bool)
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type point struct {
	X int32
	Y int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

var (
	procGetWindowRect         = user32.NewProc("GetWindowRect")
	procGetClientRect         = user32.NewProc("GetClientRect")
	procClientToScreen        = user32.NewProc("ClientToScreen")
	procMonitorFromWindow     = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW       = user32.NewProc("GetMonitorInfoW")
	procSendMessageW          = user32.NewProc("SendMessageW")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procMoveWindow            = user32.NewProc("MoveWindow")
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

func postKeyToWindow(provider TargetProvider, vk uint16, keyUp bool) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	msg := wmKeyDown
	lParam := uintptr(1)
	if keyUp {
		msg = wmKeyUp
		lParam = 0xC0000001
	}

	_, _, err := procPostMessageW.Call(
		uintptr(handle),
		uintptr(msg),
		uintptr(vk),
		lParam,
	)
	if err != syscall.Errno(0) {
		return err
	}

	return nil
}

func postMouseMove(provider TargetProvider, clientX, clientY int32) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	lParam := uintptr(uint32(uint16(clientX)) | uint32(uint16(clientY))<<16)
	_, _, err := procPostMessageW.Call(
		uintptr(handle),
		uintptr(wmMouseMove),
		0,
		lParam,
	)
	if err != syscall.Errno(0) {
		return err
	}

	return nil
}

func postMouseButton(provider TargetProvider, button string, down bool, clientX, clientY int32) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	msg := wmLButtonDown
	wParam := uintptr(mkLButton)
	if button == "right" {
		msg = wmRButtonDown
		wParam = uintptr(mkRButton)
	}
	if !down {
		if button == "right" {
			msg = wmRButtonUp
		} else {
			msg = wmLButtonUp
		}
		wParam = 0
	}

	lParam := uintptr(uint32(uint16(clientX)) | uint32(uint16(clientY))<<16)
	_, _, err := procPostMessageW.Call(
		uintptr(handle),
		uintptr(msg),
		wParam,
		lParam,
	)
	if err != syscall.Errno(0) {
		return err
	}

	return nil
}

func postMouseWheel(provider TargetProvider, delta int32, clientX, clientY int32) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	// WM_MOUSEWHEEL expects screen coordinates in lParam.
	screenX, screenY := clientToScreen(provider, clientX, clientY)
	lParam := uintptr(uint32(uint16(screenX)) | uint32(uint16(screenY))<<16)
	wParam := uintptr(uint32(int16(delta)) << 16)

	_, _, err := procSendMessageW.Call(
		uintptr(handle),
		uintptr(wmMouseWheel),
		wParam,
		lParam,
	)
	if err != syscall.Errno(0) {
		return err
	}

	return nil
}

func clientToScreen(provider TargetProvider, clientX, clientY int32) (int32, int32) {
	if rect, ok := currentClientScreenRect(provider); ok {
		return rect.Left + clientX, rect.Top + clientY
	}
	return clientX, clientY
}

func postText(provider TargetProvider, text string) error {
	if provider == nil {
		return nil
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return nil
	}

	for _, unit := range utf16.Encode([]rune(text)) {
		_, _, err := procSendMessageW.Call(
			uintptr(handle),
			uintptr(wmChar),
			uintptr(unit),
			0,
		)
		if err != syscall.Errno(0) {
			return err
		}
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

	workArea, ok := currentMonitorWorkArea(handle)
	if ok {
		maxClientWidth := int(workArea.Right-workArea.Left) - int(frameWidth)
		maxClientHeight := int(workArea.Bottom-workArea.Top) - int(frameHeight)
		desiredClientWidth, desiredClientHeight = fitClientSize(desiredClientWidth, desiredClientHeight, maxClientWidth, maxClientHeight)
	}

	desiredClientWidth = makeEvenDimension(desiredClientWidth)
	desiredClientHeight = makeEvenDimension(desiredClientHeight)

	newWidth := clampDimension(desiredClientWidth + int(frameWidth))
	newHeight := clampDimension(desiredClientHeight + int(frameHeight))
	newLeft := windowRect.Left
	newTop := windowRect.Top

	if ok {
		newLeft = clampInt32(newLeft, workArea.Left, workArea.Right-int32(newWidth))
		newTop = clampInt32(newTop, workArea.Top, workArea.Bottom-int32(newHeight))
	}

	result, _, err := procMoveWindow.Call(
		uintptr(handle),
		uintptr(newLeft),
		uintptr(newTop),
		uintptr(newWidth),
		uintptr(newHeight),
		1,
	)
	if result == 0 && err != syscall.Errno(0) {
		return err
	}

	return nil
}

func currentMonitorWorkArea(handle uint64) (rect, bool) {
	monitor, _, _ := procMonitorFromWindow.Call(uintptr(handle), monitorDefaultToNearest)
	if monitor == 0 {
		return rect{}, false
	}

	info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	result, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return rect{}, false
	}

	return info.RcWork, true
}

func currentClientRect(handle uint64) (rect, bool) {
	var out rect
	result, _, _ := procGetClientRect.Call(uintptr(handle), uintptr(unsafe.Pointer(&out)))
	if result == 0 {
		return rect{}, false
	}

	return out, true
}

func currentClientScreenRect(provider TargetProvider) (rect, bool) {
	if provider == nil {
		return rect{}, false
	}

	handle, ok := provider.CurrentHandle()
	if !ok || handle == 0 {
		return rect{}, false
	}

	clientRect, ok := currentClientRect(handle)
	if !ok {
		return rect{}, false
	}

	clientOrigin := point{}
	result, _, _ := procClientToScreen.Call(uintptr(handle), uintptr(unsafe.Pointer(&clientOrigin)))
	if result == 0 {
		return rect{}, false
	}

	return rect{
		Left:   clientOrigin.X,
		Top:    clientOrigin.Y,
		Right:  clientOrigin.X + (clientRect.Right - clientRect.Left),
		Bottom: clientOrigin.Y + (clientRect.Bottom - clientRect.Top),
	}, true
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

func fitClientSize(desiredWidth, desiredHeight, maxWidth, maxHeight int) (int, int) {
	desiredWidth = clampDimension(desiredWidth)
	desiredHeight = clampDimension(desiredHeight)
	if maxWidth < 1 {
		maxWidth = 1
	}
	if maxHeight < 1 {
		maxHeight = 1
	}

	scaleX := float64(maxWidth) / float64(desiredWidth)
	scaleY := float64(maxHeight) / float64(desiredHeight)
	scale := math.Min(1, math.Min(scaleX, scaleY))
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return desiredWidth, desiredHeight
	}

	width := int(math.Round(float64(desiredWidth) * scale))
	height := int(math.Round(float64(desiredHeight) * scale))

	return makeEvenDimension(clampDimension(width)), makeEvenDimension(clampDimension(height))
}

func clampInt32(value, minValue, maxValue int32) int32 {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func makeEvenDimension(value int) int {
	value = clampDimension(value)
	if value%2 != 0 {
		value--
	}
	if value < 200 {
		return 200
	}
	return value
}
