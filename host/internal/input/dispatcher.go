package input

import (
	"encoding/json"
	"fmt"
	"log"
)

type Injector interface {
	Move(x, y float64) error
	Tap(button string, x, y float64) error
	MouseDown(button string, x, y float64) error
	MouseUp(button string, x, y float64) error
	Scroll(deltaX, deltaY, x, y float64) error
	ResizeViewport(command Command) error
	KeyDown(command Command) error
	KeyUp(command Command) error
	Text(text string) error
}

type Dispatcher struct {
	injector Injector
}

func NewDispatcher(injector Injector) *Dispatcher {
	return &Dispatcher{injector: injector}
}

func (d *Dispatcher) Dispatch(raw []byte) error {
	var command Command
	if err := json.Unmarshal(raw, &command); err != nil {
		return err
	}

	log.Printf("input: %s", command.Type)

	switch command.Type {
	case "input.tap":
		return d.injector.Tap(command.Button, command.X, command.Y)
	case "input.mouseMove":
		return d.injector.Move(command.X, command.Y)
	case "input.mouseDown":
		return d.injector.MouseDown(command.Button, command.X, command.Y)
	case "input.mouseUp":
		return d.injector.MouseUp(command.Button, command.X, command.Y)
	case "input.scroll":
		return d.injector.Scroll(command.DeltaX, command.DeltaY, command.X, command.Y)
	case "viewport.resize":
		return d.injector.ResizeViewport(command)
	case "input.keyDown":
		return d.injector.KeyDown(command)
	case "input.keyUp":
		return d.injector.KeyUp(command)
	case "input.text":
		return d.injector.Text(command.Text)
	default:
		return fmt.Errorf("unsupported input command: %s", command.Type)
	}
}
