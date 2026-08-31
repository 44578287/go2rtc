package xiaomi

import (
	"errors"
	"strings"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/xiaomi/miss"
)

func DialPTZ(rawURL string) (core.PTZController, func(), error) {
	if strings.Contains(rawURL, "vendor") {
		return miss.DialPTZ(rawURL)
	}
	return nil, nil, errors.New("xiaomi: PTZ unsupported for legacy source")
}
