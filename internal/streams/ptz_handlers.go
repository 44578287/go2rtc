package streams

import (
	"errors"
	"strings"

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

func HasPTZHandler(source string) bool {
	if i := strings.IndexByte(source, ':'); i > 0 {
		scheme := source[:i]
		if _, ok := ptzHandlers[scheme]; ok {
			return true
		}
		if _, ok := redirects[scheme]; ok {
			return true
		}
	}
	return false
}

var _ = errors.Is // keep imports stable if handler error policy expands
