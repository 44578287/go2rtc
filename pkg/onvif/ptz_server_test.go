package onvif

import (
	"strings"
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

func TestPTZFindTagAttribute(t *testing.T) {
	body := []byte(`<tptz:ContinuousMove><tptz:Velocity><tt:PanTilt x="-0.75" y='0.25'/><tt:Zoom x="0.5"/></tptz:Velocity></tptz:ContinuousMove>`)

	pan, err := FindTagFloatAttribute(body, "PanTilt", "x")
	if err != nil || pan != -0.75 {
		t.Fatalf("pan=%v err=%v", pan, err)
	}
	tilt, err := FindTagFloatAttribute(body, "PanTilt", "y")
	if err != nil || tilt != 0.25 {
		t.Fatalf("tilt=%v err=%v", tilt, err)
	}
	zoom, err := FindTagFloatAttribute(body, "Zoom", "x")
	if err != nil || zoom != 0.5 {
		t.Fatalf("zoom=%v err=%v", zoom, err)
	}
}

func TestPTZParseTimeout(t *testing.T) {
	d, err := ParsePTZTimeout("PT0.5S")
	if err != nil || d != 500*time.Millisecond {
		t.Fatalf("duration=%v err=%v", d, err)
	}
	if _, err = ParsePTZTimeout("P1D"); err == nil {
		t.Fatal("expected unsupported duration error")
	}
}

func TestPTZResponses(t *testing.T) {
	status := string(PTZGetStatusResponse(core.PTZStatus{
		PanTilt: core.PTZMoveStatusMoving,
		Zoom:    core.PTZMoveStatusUnknown,
	}))
	if !strings.Contains(status, `<tt:PanTilt>MOVING</tt:PanTilt>`) {
		t.Fatalf("missing move status: %s", status)
	}

	nodes := string(PTZGetNodesResponse())
	if !strings.Contains(nodes, ptzVelocitySpace) {
		t.Fatalf("missing velocity space: %s", nodes)
	}
}
