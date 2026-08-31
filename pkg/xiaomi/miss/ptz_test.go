package miss

import "testing"

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
