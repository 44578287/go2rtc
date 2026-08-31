package streams

import (
	"strings"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

// PTZHandler opens a control-only PTZ session for a source without requiring
// its media producer to be started. The returned cleanup function must release
// the temporary control session.
type PTZHandler func(source string) (controller core.PTZController, cleanup func(), err error)

var ptzHandlers = map[string]PTZHandler{}

func HandlePTZFunc(scheme string, handler PTZHandler) {
	ptzHandlers[scheme] = handler
}

func GetPTZController(source string) (core.PTZController, func(), error) {
	if i := strings.IndexByte(source, ':'); i > 0 {
		scheme := source[:i]

		if redirect, ok := redirects[scheme]; ok {
			location, err := redirect(source)
			if err != nil {
				return nil, nil, err
			}
			if location != "" {
				return GetPTZController(location)
			}
		}

		if handler, ok := ptzHandlers[scheme]; ok {
			controller, cleanup, err := handler(source)
			if cleanup == nil {
				cleanup = func() {}
			}
			return controller, cleanup, err
		}
	}

	return nil, nil, ErrPTZUnsupported
}

type ptzControlSession struct {
	producer   *Producer
	source     string
	controller core.PTZController
	cleanup    func()
	timer      *time.Timer
	mu         sync.Mutex
}

var (
	ptzControlSessionsMu sync.Mutex
	ptzControlSessions   = map[*Producer]*ptzControlSession{}
)

// acquirePTZControlSession keeps a short-lived control-only connection so an
// ONVIF ContinuousMove followed by Stop uses the same encrypted camera session.
// No video stream is started by this path.
func acquirePTZControlSession(producer *Producer, source string) (*ptzControlSession, error) {
	ptzControlSessionsMu.Lock()
	defer ptzControlSessionsMu.Unlock()

	if session := ptzControlSessions[producer]; session != nil {
		if session.source == source {
			if session.timer != nil {
				session.timer.Stop()
				session.timer = nil
			}
			return session, nil
		}
		releasePTZControlSessionLocked(producer, session)
	}

	controller, cleanup, err := GetPTZController(source)
	if err != nil {
		return nil, err
	}

	session := &ptzControlSession{
		producer: producer, source: source, controller: controller, cleanup: cleanup,
	}
	ptzControlSessions[producer] = session
	return session, nil
}

func renewPTZControlSession(session *ptzControlSession, lease time.Duration) {
	if lease <= 0 {
		lease = 2 * time.Second
	}

	ptzControlSessionsMu.Lock()
	defer ptzControlSessionsMu.Unlock()

	if ptzControlSessions[session.producer] != session {
		return
	}
	if session.timer != nil {
		session.timer.Stop()
	}
	session.timer = time.AfterFunc(lease, func() {
		ptzControlSessionsMu.Lock()
		defer ptzControlSessionsMu.Unlock()
		if ptzControlSessions[session.producer] == session {
			releasePTZControlSessionLocked(session.producer, session)
		}
	})
}

func releasePTZControlSession(producer *Producer) {
	ptzControlSessionsMu.Lock()
	defer ptzControlSessionsMu.Unlock()
	if session := ptzControlSessions[producer]; session != nil {
		releasePTZControlSessionLocked(producer, session)
	}
}

func releasePTZControlSessionLocked(producer *Producer, session *ptzControlSession) {
	delete(ptzControlSessions, producer)
	if session.timer != nil {
		session.timer.Stop()
		session.timer = nil
	}
	if session.cleanup != nil {
		session.cleanup()
	}
}
