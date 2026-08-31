package core

import "time"

const (
	PTZMoveStatusUnknown = "UNKNOWN"
	PTZMoveStatusIdle    = "IDLE"
	PTZMoveStatusMoving  = "MOVING"
)

// PTZCapabilities describes optional pan/tilt/zoom features exposed by a source.
// A source implements PTZController in addition to Producer when it supports PTZ.
type PTZCapabilities struct {
	Pan            bool `json:"pan"`
	Tilt           bool `json:"tilt"`
	Zoom           bool `json:"zoom"`
	ContinuousMove bool `json:"continuous_move"`
	RelativeMove   bool `json:"relative_move"`
	AbsoluteMove   bool `json:"absolute_move"`
	Presets        bool `json:"presets"`
}

// PTZMove uses normalized ONVIF-style velocities in the -1..1 range.
// Timeout is optional. A controller that supports it should stop the motor when
// the timeout expires so a lost client cannot leave a camera moving forever.
type PTZMove struct {
	Pan     float64       `json:"pan"`
	Tilt    float64       `json:"tilt"`
	Zoom    float64       `json:"zoom"`
	Timeout time.Duration `json:"-"`
}

type PTZStatus struct {
	PanTilt string `json:"pan_tilt"`
	Zoom    string `json:"zoom"`
}

// PTZController is deliberately optional and is not embedded in Producer.
// This keeps all existing source implementations source-compatible.
type PTZController interface {
	PTZCapabilities() PTZCapabilities
	PTZContinuousMove(move PTZMove) error
	PTZStop() error
	PTZStatus() PTZStatus
}
