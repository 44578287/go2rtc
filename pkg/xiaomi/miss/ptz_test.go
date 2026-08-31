package miss

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestPTZDafangMove(t *testing.T) {
	tests := []struct {
		name             string
		pan, tilt        float64
		horizontal       byte
		vertical         byte
		speed            byte
		stop             bool
	}{
		{"left", -1, 0, 1, 0, 9, false},
		{"right-half", 0.5, 0, 2, 0, 5, false},
		{"up", 0, 1, 0, 2, 9, false},
		{"down", 0, -0.2, 0, 1, 2, false},
		{"dominant-tilt", 0.2, 0.9, 0, 2, 9, false},
		{"clamp", 2, 0, 2, 0, 9, false},
		{"stop", 0, 0, 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, v, speed, stop := dafangPTZMove(tt.pan, tt.tilt)
			if h != tt.horizontal || v != tt.vertical || speed != tt.speed || stop != tt.stop {
				t.Fatalf("got h=%d v=%d speed=%d stop=%v; want h=%d v=%d speed=%d stop=%v",
					h, v, speed, stop, tt.horizontal, tt.vertical, tt.speed, tt.stop)
			}
		})
	}
}

func TestPTZModernMove(t *testing.T) {
	tests := []struct {
		name      string
		pan, tilt float64
		direction string
		speed     int
		stop      bool
	}{
		{"left", -1, 0, "left", 100, false},
		{"right-half", 0.5, 0, "right", 50, false},
		{"up", 0, 1, "up", 100, false},
		{"down", 0, -0.2, "down", 20, false},
		{"dominant-tilt", 0.2, 0.9, "up", 90, false},
		{"clamp", 2, 0, "right", 100, false},
		{"stop", 0, 0, "stop", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direction, speed, stop := modernPTZMove(tt.pan, tt.tilt)
			if direction != tt.direction || speed != tt.speed || stop != tt.stop {
				t.Fatalf("got direction=%q speed=%d stop=%v; want direction=%q speed=%d stop=%v",
					direction, speed, stop, tt.direction, tt.speed, tt.stop)
			}
		})
	}
}

func TestPTZModernCommand(t *testing.T) {
	cmd := modernPTZCommand("left", 37)
	if len(cmd) < 5 {
		t.Fatalf("command too short: %x", cmd)
	}
	if got := binary.BigEndian.Uint32(cmd[:4]); got != cmdMotorReq {
		t.Fatalf("command id=%#x want %#x", got, cmdMotorReq)
	}
	if got := string(cmd[4:]); got != `{"direction":"left","speed":37}` {
		t.Fatalf("payload=%q", got)
	}

	stop := modernPTZCommand("stop", 0)
	if !strings.Contains(string(stop[4:]), `"direction":"stop"`) || !strings.Contains(string(stop[4:]), `"speed":0`) {
		t.Fatalf("stop payload=%q", stop[4:])
	}
}

func TestPTZModernModelGate(t *testing.T) {
	for _, model := range []string{ModelC200, "xiaomi.camera.c01a01", "chuangmi.camera.069a01"} {
		if !modernPTZSupported(model) {
			t.Fatalf("expected model %s to support modern PTZ", model)
		}
	}
	if modernPTZSupported("isa.camera.hlc6") {
		t.Fatal("fixed/unknown model must not advertise PTZ")
	}
}
