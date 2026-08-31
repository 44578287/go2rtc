package miss

import "github.com/AlexxIT/go2rtc/pkg/core"

// DialPTZ establishes only the encrypted Xiaomi control session. It does not
// send StartMedia or probe video/audio, so PTZ can be used without consuming a
// camera media stream slot.
func DialPTZ(rawURL string) (core.PTZController, func(), error) {
	client, err := NewClient(rawURL)
	if err != nil {
		return nil, nil, err
	}

	controller := &Producer{client: client}
	cleanup := func() {
		_ = client.Close()
	}
	return controller, cleanup, nil
}
