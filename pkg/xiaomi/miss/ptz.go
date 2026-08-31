package miss

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

const xiaomiPTZSafetyTimeout = 10 * time.Second

type ptzState struct {
	mu         sync.Mutex
	timer      *time.Timer
	generation uint64
	moveStatus string
}

func (p *Producer) PTZCapabilities() core.PTZCapabilities {
	switch p.client.model {
	case ModelDafang, ModelXiaofang:
		return core.PTZCapabilities{
			Pan:            true,
			Tilt:           true,
			ContinuousMove: true,
		}
	default:
		return core.PTZCapabilities{}
	}
}

func (p *Producer) PTZContinuousMove(move core.PTZMove) error {
	switch p.client.model {
	case ModelDafang, ModelXiaofang:
	default:
		return fmt.Errorf("xiaomi: PTZ unsupported for model %s", p.client.model)
	}

	if move.Zoom != 0 {
		return fmt.Errorf("xiaomi: PTZ zoom unsupported for model %s", p.client.model)
	}

	horizontal, vertical, speed, stop := dafangPTZMove(move.Pan, move.Tilt)
	if stop {
		return p.PTZStop()
	}

	p.ptz.mu.Lock()
	defer p.ptz.mu.Unlock()

	if p.ptz.timer != nil {
		p.ptz.timer.Stop()
		p.ptz.timer = nil
	}

	if err := p.client.WriteCommand(dafangRaw(0xff2404, horizontal, vertical, speed)); err != nil {
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
	switch p.client.model {
	case ModelDafang, ModelXiaofang:
		if err := p.client.WriteCommand(dafangRaw(0xff2404, 0, 0, 5)); err != nil {
			p.ptz.moveStatus = core.PTZMoveStatusUnknown
			return err
		}
		p.ptz.moveStatus = core.PTZMoveStatusIdle
		return nil
	default:
		return fmt.Errorf("xiaomi: PTZ unsupported for model %s", p.client.model)
	}
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
