package input

type Command struct {
	Type             string  `json:"type"`
	Button           string  `json:"button,omitempty"`
	Key              string  `json:"key,omitempty"`
	Code             string  `json:"code,omitempty"`
	Text             string  `json:"text,omitempty"`
	X                float64 `json:"x,omitempty"`
	Y                float64 `json:"y,omitempty"`
	DeltaX           float64 `json:"deltaX,omitempty"`
	DeltaY           float64 `json:"deltaY,omitempty"`
	Width            int     `json:"width,omitempty"`
	Height           int     `json:"height,omitempty"`
	DevicePixelRatio float64 `json:"devicePixelRatio,omitempty"`
	AltKey           bool    `json:"altKey,omitempty"`
	CtrlKey          bool    `json:"ctrlKey,omitempty"`
	ShiftKey         bool    `json:"shiftKey,omitempty"`
	MetaKey          bool    `json:"metaKey,omitempty"`
}
