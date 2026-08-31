package onvif

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

const (
	PTZGetNodes                    = "GetNodes"
	PTZGetNode                     = "GetNode"
	PTZGetConfigurations           = "GetConfigurations"
	PTZGetConfiguration            = "GetConfiguration"
	PTZGetConfigurationOptions     = "GetConfigurationOptions"
	PTZGetCompatibleConfigurations = "GetCompatibleConfigurations"
	PTZContinuousMove              = "ContinuousMove"
	PTZStop                        = "Stop"
	PTZGetStatus                   = "GetStatus"
)

const (
	ptzNodeToken = "go2rtc-ptz-node"
	ptzVelocitySpace = "http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace"
)

func PTZGetServiceCapabilitiesResponse() []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetServiceCapabilitiesResponse>
	<tptz:Capabilities EFlip="false" Reverse="false" GetCompatibleConfigurations="true" MoveStatus="true" StatusPosition="false" />
</tptz:GetServiceCapabilitiesResponse>`)
	return e.Bytes()
}

func PTZGetNodesResponse() []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetNodesResponse>`)
	appendPTZNode(e, "PTZNode")
	e.Append(`</tptz:GetNodesResponse>`)
	return e.Bytes()
}

func PTZGetNodeResponse() []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetNodeResponse>`)
	appendPTZNode(e, "PTZNode")
	e.Append(`</tptz:GetNodeResponse>`)
	return e.Bytes()
}

func appendPTZNode(e *Envelope, tag string) {
	e.Appendf(`<tptz:%s token="%s" FixedHomePosition="false">
	<tt:Name>go2rtc PTZ</tt:Name>
	<tt:SupportedPTZSpaces>
		<tt:ContinuousPanTiltVelocitySpace>
			<tt:URI>%s</tt:URI>
			<tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange>
			<tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange>
		</tt:ContinuousPanTiltVelocitySpace>
	</tt:SupportedPTZSpaces>
	<tt:MaximumNumberOfPresets>0</tt:MaximumNumberOfPresets>
	<tt:HomeSupported>false</tt:HomeSupported>
</tptz:%s>`, tag, ptzNodeToken, ptzVelocitySpace, tag)
}

func PTZGetConfigurationsResponse(tokens []string) []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetConfigurationsResponse>`)
	for _, token := range tokens {
		appendPTZServiceConfiguration(e, "PTZConfiguration", token)
	}
	e.Append(`</tptz:GetConfigurationsResponse>`)
	return e.Bytes()
}

func PTZGetConfigurationResponse(token string) []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetConfigurationResponse>`)
	appendPTZServiceConfiguration(e, "PTZConfiguration", token)
	e.Append(`</tptz:GetConfigurationResponse>`)
	return e.Bytes()
}

func PTZGetCompatibleConfigurationsResponse(token string) []byte {
	e := NewEnvelope()
	e.Append(`<tptz:GetCompatibleConfigurationsResponse>`)
	appendPTZServiceConfiguration(e, "PTZConfiguration", token)
	e.Append(`</tptz:GetCompatibleConfigurationsResponse>`)
	return e.Bytes()
}

func appendPTZServiceConfiguration(e *Envelope, tag, token string) {
	e.Appendf(`<tptz:%s token="%s">
	<tt:Name>PTZ %s</tt:Name>
	<tt:UseCount>1</tt:UseCount>
	<tt:NodeToken>%s</tt:NodeToken>
	<tt:DefaultContinuousPanTiltVelocitySpace>%s</tt:DefaultContinuousPanTiltVelocitySpace>
	<tt:DefaultPTZTimeout>PT1S</tt:DefaultPTZTimeout>
</tptz:%s>`, tag, token, token, ptzNodeToken, ptzVelocitySpace, tag)
}

// AppendProfilePTZConfiguration adds a PTZ configuration to a Media profile.
// The profile token is also used as the PTZ configuration token so the ONVIF
// server can map commands directly back to a go2rtc stream name.
func AppendProfilePTZConfiguration(e *Envelope, token string) {
	e.Appendf(`<tt:PTZConfiguration token="%s">
	<tt:Name>PTZ %s</tt:Name>
	<tt:UseCount>1</tt:UseCount>
	<tt:NodeToken>%s</tt:NodeToken>
	<tt:DefaultContinuousPanTiltVelocitySpace>%s</tt:DefaultContinuousPanTiltVelocitySpace>
	<tt:DefaultPTZTimeout>PT1S</tt:DefaultPTZTimeout>
</tt:PTZConfiguration>`, token, token, ptzNodeToken, ptzVelocitySpace)
}

func PTZGetConfigurationOptionsResponse() []byte {
	e := NewEnvelope()
	e.Appendf(`<tptz:GetConfigurationOptionsResponse>
	<tptz:PTZConfigurationOptions>
		<tt:Spaces>
			<tt:ContinuousPanTiltVelocitySpace>
				<tt:URI>%s</tt:URI>
				<tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange>
				<tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange>
			</tt:ContinuousPanTiltVelocitySpace>
		</tt:Spaces>
		<tt:PTZTimeout><tt:Min>PT0S</tt:Min><tt:Max>PT60S</tt:Max></tt:PTZTimeout>
	</tptz:PTZConfigurationOptions>
</tptz:GetConfigurationOptionsResponse>`, ptzVelocitySpace)
	return e.Bytes()
}

func PTZContinuousMoveResponse() []byte {
	e := NewEnvelope()
	e.Append(`<tptz:ContinuousMoveResponse />`)
	return e.Bytes()
}

func PTZStopResponse() []byte {
	e := NewEnvelope()
	e.Append(`<tptz:StopResponse />`)
	return e.Bytes()
}

func PTZGetStatusResponse(status core.PTZStatus) []byte {
	e := NewEnvelope()
	e.Appendf(`<tptz:GetStatusResponse>
	<tptz:PTZStatus>
		<tt:MoveStatus><tt:PanTilt>%s</tt:PanTilt><tt:Zoom>%s</tt:Zoom></tt:MoveStatus>
		<tt:Error></tt:Error>
		<tt:UtcTime>%s</tt:UtcTime>
	</tptz:PTZStatus>
</tptz:GetStatusResponse>`, status.PanTilt, status.Zoom, time.Now().UTC().Format(time.RFC3339Nano))
	return e.Bytes()
}

func FindTagAttribute(b []byte, tag, attr string) string {
	pattern := fmt.Sprintf(`(?s)<(?:\w+:)?%s\b[^>]*\b%s\s*=\s*["']([^"']+)["']`, regexp.QuoteMeta(tag), regexp.QuoteMeta(attr))
	m := regexp.MustCompile(pattern).FindSubmatch(b)
	if len(m) != 2 {
		return ""
	}
	return string(m[1])
}

func FindTagFloatAttribute(b []byte, tag, attr string) (float64, error) {
	s := FindTagAttribute(b, tag, attr)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

func ParsePTZTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if !strings.HasPrefix(s, "PT") || !strings.HasSuffix(s, "S") {
		return 0, fmt.Errorf("onvif: unsupported PTZ timeout %q", s)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(s, "PT"), "S"), 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("onvif: invalid PTZ timeout %q", s)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func FaultResponse(reason string) []byte {
	e := NewEnvelope()
	e.Appendf(`<s:Fault>
	<s:Code><s:Value>s:Receiver</s:Value></s:Code>
	<s:Reason><s:Text xml:lang="en">%s</s:Text></s:Reason>
</s:Fault>`, html.EscapeString(reason))
	return e.Bytes()
}
