package input

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfKeyUp   = 0x0002
	keyeventfScancode = 0x0008
	keyeventfUnicode = 0x0004

	mouseeventfMove        = 0x0001
	mouseeventfAbsolute    = 0x8000
	mouseeventfLeftDown    = 0x0002
	mouseeventfLeftUp      = 0x0004
	mouseeventfRightDown   = 0x0008
	mouseeventfRightUp     = 0x0010
	mouseeventfWheel       = 0x0800
	mouseeventfVirtualDesk = 0x4000

	smCxScreen = 0
	smCyScreen = 1

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

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSendInput        = user32.NewProc("SendInput")
	procSetCursorPos     = user32.NewProc("SetCursorPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type inputUnion struct {
	Mouse mouseInput
}

type inputPacket struct {
	Type uint32
	_    uint32
	Ki   keyboardInput
}

type mousePacket struct {
	Type uint32
	_    uint32
	Mi   mouseInput
}

type SendInputInjector struct {
	targets TargetProvider
}

func NewSendInputInjector(targets TargetProvider) *SendInputInjector {
	return &SendInputInjector{targets: targets}
}

func (s *SendInputInjector) Move(x, y float64) error {
	screenX, screenY := normalizePoint(x, y, s.targets)
	_, _, err := procSetCursorPos.Call(uintptr(screenX), uintptr(screenY))
	if err != syscall.Errno(0) {
		return err
	}

	return nil
}

func (s *SendInputInjector) Tap(button string, x, y float64) error {
	if err := s.Move(x, y); err != nil {
		return err
	}

	if err := s.MouseDown(button, x, y); err != nil {
		return err
	}

	return s.MouseUp(button, x, y)
}

func (s *SendInputInjector) MouseDown(button string, x, y float64) error {
	if err := s.Move(x, y); err != nil {
		return err
	}

	return sendMouse(buttonToFlag(button, true), 0)
}

func (s *SendInputInjector) MouseUp(button string, x, y float64) error {
	if err := s.Move(x, y); err != nil {
		return err
	}

	return sendMouse(buttonToFlag(button, false), 0)
}

func (s *SendInputInjector) Scroll(_, deltaY float64) error {
	if s.targets != nil {
		if handle, ok := s.targets.CurrentHandle(); ok && handle != 0 {
			return postMouseWheel(s.targets, int32(-deltaY*120))
		}
	}
	return sendMouse(mouseeventfWheel, uint32(int32(-deltaY*120)))
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
	focusTargetWindow(s.targets)
	if vk, ok := specialKeyToVK(command.Key); ok {
		return sendVirtualKeyboard(vk, 0)
	}
	return nil
}

func (s *SendInputInjector) KeyUp(command Command) error {
	if vk, ok := specialKeyToVK(command.Key); ok {
		return sendVirtualKeyboard(vk, keyeventfKeyUp)
	}
	return nil
}

func (s *SendInputInjector) Text(text string) error {
	focusTargetWindow(s.targets)
	if s.targets != nil {
		if handle, ok := s.targets.CurrentHandle(); ok && handle != 0 {
			return postText(s.targets, text)
		}
	}
	for _, unit := range utf16.Encode([]rune(text)) {
		if err := sendUnicodeKeyboard(unit, 0); err != nil {
			return err
		}
		if err := sendUnicodeKeyboard(unit, keyeventfKeyUp); err != nil {
			return err
		}
	}

	return nil
}

func sendMouse(flags uint32, data uint32) error {
	packet := mousePacket{
		Type: inputMouse,
		Mi: mouseInput{
			DwFlags:   flags,
			MouseData: data,
		},
	}

	result, _, err := procSendInput.Call(
		1,
		uintptr(unsafe.Pointer(&packet)),
		unsafe.Sizeof(packet),
	)
	if result == 0 {
		return err
	}

	return nil
}

func sendUnicodeKeyboard(scan uint16, flags uint32) error {
	packet := inputPacket{
		Type: inputKeyboard,
		Ki: keyboardInput{
			WScan:   scan,
			DwFlags: keyeventfUnicode | flags,
		},
	}

	result, _, err := procSendInput.Call(
		1,
		uintptr(unsafe.Pointer(&packet)),
		unsafe.Sizeof(packet),
	)
	if result == 0 {
		return err
	}

	return nil
}

func sendVirtualKeyboard(vk uint16, flags uint32) error {
	packet := inputPacket{
		Type: inputKeyboard,
		Ki: keyboardInput{
			WVk:     vk,
			DwFlags: flags &^ keyeventfUnicode &^ keyeventfScancode,
		},
	}

	result, _, err := procSendInput.Call(
		1,
		uintptr(unsafe.Pointer(&packet)),
		unsafe.Sizeof(packet),
	)
	if result == 0 {
		return err
	}

	return nil
}

func buttonToFlag(button string, down bool) uint32 {
	switch button {
	case "right":
		if down {
			return mouseeventfRightDown
		}
		return mouseeventfRightUp
	default:
		if down {
			return mouseeventfLeftDown
		}
		return mouseeventfLeftUp
	}
}

func normalizeToScreen(x, y float64) (int32, int32) {
	width, _, _ := procGetSystemMetrics.Call(smCxScreen)
	height, _, _ := procGetSystemMetrics.Call(smCyScreen)
	return int32(float64(width) * x), int32(float64(height) * y)
}

func normalizePoint(x, y float64, targets TargetProvider) (int32, int32) {
	if rect, ok := currentClientScreenRect(targets); ok {
		width := rect.Right - rect.Left
		height := rect.Bottom - rect.Top
		return rect.Left + int32(float64(width)*clampUnit(x)), rect.Top + int32(float64(height)*clampUnit(y))
	}

	return normalizeToScreen(x, y)
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
