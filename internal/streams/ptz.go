package streams

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

var (
	ErrPTZStreamNotFound = errors.New("streams: PTZ stream not found")
	ErrPTZUnsupported    = errors.New("streams: PTZ unsupported")
)

type PTZInfo struct {
	Capabilities core.PTZCapabilities `json:"capabilities"`
	Status       core.PTZStatus       `json:"status"`
}

// withPTZ reuses the current producer session when one is already dialed. For
// inactive streams it opens a source-specific control-only session so PTZ does
// not consume a media session or leave an unread media queue behind.
func (p *Producer) withPTZ(fn func(core.PTZController) error) (bool, error) {
	p.mu.Lock()
	if p.conn != nil {
		if controller, ok := p.conn.(core.PTZController); ok {
			err := fn(controller)
			p.mu.Unlock()
			return true, err
		}
	}
	source := p.url
	p.mu.Unlock()

	controller, cleanup, err := GetPTZController(source)
	if errors.Is(err, ErrPTZUnsupported) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer cleanup()

	return true, fn(controller)
}

func (s *Stream) withPTZ(fn func(core.PTZController) error) error {
	s.mu.Lock()
	producers := append([]*Producer(nil), s.producers...)
	s.mu.Unlock()

	var lastErr error
	for _, producer := range producers {
		ok, err := producer.withPTZ(fn)
		if ok {
			return err
		}
		if err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return ErrPTZUnsupported
}

func PTZGetInfo(name string) (*PTZInfo, error) {
	stream := Get(name)
	if stream == nil {
		return nil, ErrPTZStreamNotFound
	}

	info := new(PTZInfo)
	err := stream.withPTZ(func(controller core.PTZController) error {
		info.Capabilities = controller.PTZCapabilities()
		if !ptzCapabilitiesSupported(info.Capabilities) {
			return ErrPTZUnsupported
		}
		info.Status = controller.PTZStatus()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

func PTZContinuousMove(name string, move core.PTZMove) error {
	stream := Get(name)
	if stream == nil {
		return ErrPTZStreamNotFound
	}
	return stream.withPTZ(func(controller core.PTZController) error {
		if !controller.PTZCapabilities().ContinuousMove {
			return ErrPTZUnsupported
		}
		return controller.PTZContinuousMove(move)
	})
}

func PTZStop(name string) error {
	stream := Get(name)
	if stream == nil {
		return ErrPTZStreamNotFound
	}
	return stream.withPTZ(func(controller core.PTZController) error {
		if !ptzCapabilitiesSupported(controller.PTZCapabilities()) {
			return ErrPTZUnsupported
		}
		return controller.PTZStop()
	})
}

func ptzCapabilitiesSupported(c core.PTZCapabilities) bool {
	return c.Pan || c.Tilt || c.Zoom || c.ContinuousMove || c.RelativeMove || c.AbsoluteMove || c.Presets
}

type ptzAPIRequest struct {
	Source    string  `json:"src"`
	Action    string  `json:"action"`
	Pan       float64 `json:"pan"`
	Tilt      float64 `json:"tilt"`
	Zoom      float64 `json:"zoom"`
	TimeoutMS int64   `json:"timeout_ms"`
}

func apiPTZ(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		src := r.URL.Query().Get("src")
		if src == "" {
			http.Error(w, "missing src", http.StatusBadRequest)
			return
		}
		info, err := PTZGetInfo(src)
		if err != nil {
			writePTZError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(info)

	case http.MethodPost:
		var req ptzAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Source == "" {
			http.Error(w, "missing src", http.StatusBadRequest)
			return
		}

		var err error
		switch req.Action {
		case "move", "continuous_move":
			timeout := time.Duration(req.TimeoutMS) * time.Millisecond
			if timeout < 0 {
				http.Error(w, "timeout_ms must be >= 0", http.StatusBadRequest)
				return
			}
			err = PTZContinuousMove(req.Source, core.PTZMove{
				Pan: req.Pan, Tilt: req.Tilt, Zoom: req.Zoom, Timeout: timeout,
			})
		case "stop":
			err = PTZStop(req.Source)
		default:
			http.Error(w, "unsupported PTZ action", http.StatusBadRequest)
			return
		}
		if err != nil {
			writePTZError(w, err)
			return
		}
		_, _ = w.Write([]byte("{\"ok\":true}\n"))

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writePTZError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPTZStreamNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrPTZUnsupported):
		http.Error(w, err.Error(), http.StatusNotImplemented)
	default:
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
}
