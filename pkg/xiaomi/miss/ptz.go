package miss

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

const xiaomiPTZSafetyTimeout = 10 * time.Second

// modernPTZModels is intentionally explicit. The MISS motor command is shared,
// but we only advertise PTZ for camera families that are known pan/tilt models.
// Unknown models remain opt-out until their hardware behavior is verified.
var modernPTZModels = map[string]struct{}{
	"chuangmi.camera.021a04": {}, // Mi 360 Home Security Camera 2K Pro
	"chuangmi.camera.026c02": {}, // Xiaomi Smart Camera PTZ SE+
	"chuangmi.camera.029a02": {}, // Mi 360 Home Security Camera 2K
	"chuangmi.camera.039c01": {}, // Xiaomi Smart Camera 2 PTZ
	"chuangmi.camera.039a04": {}, // Xiaomi Smart Camera C400
	"chuangmi.camera.039c04": {}, // Xiaomi Smart Camera C400 (new revision)
	ModelC200:                  {}, // Xiaomi Smart Camera C200
	"chuangmi.camera.051a01": {}, // Xiaomi Smart Camera 2 AI Enhanced
	"chuangmi.camera.061a03": {}, // Xiaomi Smart Camera C500 Pro
	"chuangmi.camera.069a01": {}, // Xiaomi Smart Camera 3 PTZ
	"chuangmi.camera.079ae2": {}, // Xiaomi Smart Camera C701
	"chuangmi.camera.81ac1":  {}, // Xiaomi Smart Camera C700
	"xiaomi.camera.c01a01":   {}, // Xiaomi Smart Camera C300
	ModelC300:                  {}, // Xiaomi Smart Camera C300 Dual (CN)
}

type ptzState struct {
	mu         sync.Mutex
	timer      *time.Timer
	generation uint64
	moveStatus string
}

func (p *Producer) PTZCapabilities() core.PTZCapabilities {
	if p.client == nil {
		return core.PTZCapabilities{}
	}
	if p.client.model == ModelDafang || p.client.model == ModelXiaofang || modernPTZSupported(p.client.model) {
		return core.PTZCapabilities{
			Pan:            true,
			Tilt:           true,
			ContinuousMove: true,
		}
	}
	return core.PTZCapabilities{}
}

func (p *Producer) PTZContinuousMove(move core.PTZMove) error {
	if p.client == nil {
		return fmt.Errorf("xiaomi: PTZ control session unavailable")
	}
	if move.Zoom != 0 {
		return fmt.Errorf("xiaomi: PTZ zoom unsupported for model %s", p.client.model)
	}

	var command []byte
	switch {
	case p.client.model == ModelDafang || p.client.model == ModelXiaofang:
		horizontal, vertical, speed, stop := dafangPTZMove(move.Pan, move.Tilt)
		if stop {
			return p.PTZStop()
		}
		command = dafangRaw(0xff2404, horizontal, vertical, speed)

	case modernPTZSupported(p.client.model):
		direction, speed, stop := modernPTZMove(move.Pan, move.Tilt)
		if stop {
			return p.PTZStop()
		}
		command = modernPTZCommand(direction, speed)

	default:
		return fmt.Errorf("xiaomi: PTZ unsupported for model %s", p.client.model)
	}

	p.ptz.mu.Lock()
	defer p.ptz.mu.Unlock()

	if p.ptz.timer != nil {
		p.ptz.timer.Stop()
		p.ptz.timer = nil
	}

	if err := p.client.WriteCommand(command); err != nil {
		p.ptz.moveStatus = core.PTZMoveStatusUnknown
		return err
	}

	p.ptz.generation++
	generation := p.ptz.generation
	p.ptz.moveStatus = core.PTZMoveStatusMoving

	timeout := move.Timeout
	if timeout <= 0 || timeout > xiaomiPTZSafetyTimeout {
		timeout = xiaomiPTZSafetyTimeout
	}
	p.ptz.timer = time.AfterFunc(timeout, func() {
		p.ptzStopGeneration(generation)
	})

	return nil
}

func (p *Producer) PTZStop() error {
	p.ptz.mu.Lock()
	defer p.ptz.mu.Unlock()

	p.ptz.generation++
	if p.ptz.timer != nil {
		p.ptz.timer.Stop()
		p.ptz.timer = nil
	}

	return p.ptzStopLocked()
}

func (p *Producer) ptzStopGeneration(generation uint64) {
	p.ptz.mu.Lock()
	defer p.ptz.mu.Unlock()

	if p.ptz.generation != generation {
		return
	}

	p.ptz.generation++
	p.ptz.timer = nil
	_ = p.ptzStopLocked()
}

func (p *Producer) ptzStopLocked() error {
	if p.client == nil {
		return fmt.Errorf("xiaomi: PTZ control session unavailable")
	}

	var command []byte
	switch {
	case p.client.model == ModelDafang || p.client.model == ModelXiaofang:
		command = dafangRaw(0xff2404, 0, 0, 5)
	case modernPTZSupported(p.client.model):
		command = modernPTZCommand("stop", 0)
	default:
		return fmt.Errorf("xiaomi: PTZ unsupported for model %s", p.client.model)
	}

	if err := p.client.WriteCommand(command); err != nil {
		p.ptz.moveStatus = core.PTZMoveStatusUnknown
		return err
	}
	p.ptz.moveStatus = core.PTZMoveStatusIdle
	return nil
}

func (p *Producer) PTZStatus() core.PTZStatus {
	p.ptz.mu.Lock()
	defer p.ptz.mu.Unlock()

	status := p.ptz.moveStatus
	if status == "" {
		status = core.PTZMoveStatusUnknown
	}
	return core.PTZStatus{
		PanTilt: status,
		Zoom:    core.PTZMoveStatusUnknown,
	}
}

func modernPTZSupported(model string) bool {
	_, ok := modernPTZModels[model]
	return ok
}

// modernPTZMove maps normalized ONVIF velocity to Xiaomi MISS motor semantics:
// direction is left/right/up/down and speed is 1..100. The protocol has a
// single direction field, so V1 uses the dominant axis for diagonal vectors.
func modernPTZMove(pan, tilt float64) (direction string, speed int, stop bool) {
	pan = clampPTZ(pan)
	tilt = clampPTZ(tilt)
	if pan == 0 && tilt == 0 {
		return "stop", 0, true
	}

	magnitude := math.Abs(pan)
	if math.Abs(tilt) > magnitude {
		magnitude = math.Abs(tilt)
		if tilt > 0 {
			direction = "up"
		} else {
			direction = "down"
		}
	} else if pan < 0 {
		direction = "left"
	} else {
		direction = "right"
	}

	speed = int(math.Ceil(magnitude * 100))
	if speed < 1 {
		speed = 1
	}
	if speed > 100 {
		speed = 100
	}
	return direction, speed, false
}

// modernPTZCommand builds the plaintext carried inside Xiaomi's encrypted
// cmdEncoded (0x1001) channel. The motor request is command 0x112 followed by
// its compact JSON payload.
func modernPTZCommand(direction string, speed int) []byte {
	data := binary.BigEndian.AppendUint32(nil, cmdMotorReq)
	return fmt.Appendf(data, `{"direction":"%s","speed":%d}`, direction, speed)
}

// dafangPTZMove maps normalized ONVIF-style pan/tilt velocity to the motor
// command verified on physical Dafang hardware. V1 deliberately selects the
// dominant axis rather than assuming every firmware supports diagonal motion.
func dafangPTZMove(pan, tilt float64) (horizontal, vertical, speed byte, stop bool) {
	pan = clampPTZ(pan)
	tilt = clampPTZ(tilt)
	if pan == 0 && tilt == 0 {
		return 0, 0, 0, true
	}

	magnitude := math.Abs(pan)
	if math.Abs(tilt) > magnitude {
		magnitude = math.Abs(tilt)
		if tilt > 0 {
			vertical = 2 // up
		} else {
			vertical = 1 // down
		}
	} else if pan < 0 {
		horizontal = 1 // left; verified on hardware, old commented helper was reversed
	} else {
		horizontal = 2 // right
	}

	speed = byte(math.Ceil(magnitude * 9))
	if speed < 1 {
		speed = 1
	}
	if speed > 9 {
		speed = 9
	}
	return horizontal, vertical, speed, false
}

func clampPTZ(v float64) float64 {
	switch {
	case v < -1:
		return -1
	case v > 1:
		return 1
	default:
		return v
	}
}
