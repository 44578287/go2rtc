package xiaomi

import (
	"net/url"

	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/AlexxIT/go2rtc/pkg/core"
	pkgxiaomi "github.com/AlexxIT/go2rtc/pkg/xiaomi"
)

func init() {
	streams.HandlePTZFunc("xiaomi", func(rawURL string) (core.PTZController, func(), error) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, nil, err
		}

		if u.User != nil {
			rawURL, err = getCameraURL(u)
			if err != nil {
				return nil, nil, err
			}
		}

		return pkgxiaomi.DialPTZ(rawURL)
	})
}
