package onvif

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/internal/api"
	"github.com/AlexxIT/go2rtc/internal/app"
	"github.com/AlexxIT/go2rtc/internal/rtsp"
	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/onvif"
	"github.com/rs/zerolog"
)

func Init() {
	log = app.GetLogger("onvif")

	streams.HandleFunc("onvif", streamOnvif)

	// ONVIF server on all suburls
	api.HandleFunc("/onvif/", onvifDeviceService)

	// ONVIF client autodiscovery
	api.HandleFunc("api/onvif", apiOnvif)
}

var log zerolog.Logger

func streamOnvif(rawURL string) (core.Producer, error) {
	client, err := onvif.NewClient(rawURL)
	if err != nil {
		return nil, err
	}

	uri, err := client.GetURI()
	if err != nil {
		return nil, err
	}

	// Append hash-based arguments to the retrieved URI
	if i := strings.IndexByte(rawURL, '#'); i > 0 {
		uri += rawURL[i:]
	}

	log.Debug().Msgf("[onvif] new uri=%s", uri)

	if err = streams.Validate(uri); err != nil {
		return nil, err
	}

	return streams.GetProducer(uri)
}

func onvifDeviceService(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	operation := onvif.GetRequestAction(b)
	if operation == "" {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return
	}

	log.Trace().Msgf("[onvif] server request %s %s:\n%s", r.Method, r.RequestURI, b)

	switch operation {
	case onvif.ServiceGetServiceCapabilities:
		if strings.Contains(r.URL.Path, "ptz_service") {
			b = onvif.PTZGetServiceCapabilitiesResponse()
		} else {
			b = onvif.StaticResponse(operation)
		}

	case onvif.DeviceGetNetworkInterfaces, // important for Hass
		onvif.DeviceGetSystemDateAndTime, // important for Hass
		onvif.DeviceSetSystemDateAndTime, // return just OK
		onvif.DeviceGetDiscoveryMode,
		onvif.DeviceGetDNS,
		onvif.DeviceGetHostname,
		onvif.DeviceGetNetworkDefaultGateway,
		onvif.DeviceGetNetworkProtocols,
		onvif.DeviceGetNTP,
		onvif.DeviceGetScopes,
		onvif.MediaGetVideoEncoderConfiguration,
		onvif.MediaGetVideoEncoderConfigurations,
		onvif.MediaGetAudioEncoderConfigurations,
		onvif.MediaGetVideoEncoderConfigurationOptions,
		onvif.MediaGetAudioSources,
		onvif.MediaGetAudioSourceConfigurations:
		b = onvif.StaticResponse(operation)

	case onvif.DeviceGetCapabilities:
		b = onvif.GetCapabilitiesResponse(r.Host)

	case onvif.DeviceGetServices:
		b = onvif.GetServicesResponse(r.Host)

	case onvif.DeviceGetDeviceInformation:
		b = onvif.GetDeviceInformationResponse("", "go2rtc", app.Version, r.Host)

	case onvif.DeviceSystemReboot:
		b = onvif.StaticResponse(operation)
		time.AfterFunc(time.Second, func() { os.Exit(0) })

	case onvif.MediaGetVideoSources:
		b = onvif.GetVideoSourcesResponse(streams.GetAllNames())

	case onvif.MediaGetProfiles:
		b = onvif.GetProfilesResponse(streams.GetAllNames())

	case onvif.MediaGetProfile:
		token := onvif.FindTagValue(b, "ProfileToken")
		b = onvif.GetProfileResponse(token)

	case onvif.MediaGetVideoSourceConfigurations:
		b = onvif.GetVideoSourceConfigurationsResponse(streams.GetAllNames())

	case onvif.MediaGetVideoSourceConfiguration:
		token := onvif.FindTagValue(b, "ConfigurationToken")
		b = onvif.GetVideoSourceConfigurationResponse(token)

	case onvif.MediaGetStreamUri:
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		uri := "rtsp://" + host + ":" + rtsp.Port + "/" + onvif.FindTagValue(b, "ProfileToken")
		b = onvif.GetStreamUriResponse(uri)

	case onvif.MediaGetSnapshotUri:
		uri := "http://" + r.Host + "/api/frame.jpeg?src=" + onvif.FindTagValue(b, "ProfileToken")
		b = onvif.GetSnapshotUriResponse(uri)

	case onvif.PTZGetNodes:
		b = onvif.PTZGetNodesResponse()

	case onvif.PTZGetNode:
		b = onvif.PTZGetNodeResponse()

	case onvif.PTZGetConfigurations:
		b = onvif.PTZGetConfigurationsResponse(streams.GetAllNames())

	case onvif.PTZGetConfiguration:
		token := onvif.FindTagValue(b, "PTZConfigurationToken")
		if token == "" {
			token = onvif.FindTagValue(b, "ConfigurationToken")
		}
		b = onvif.PTZGetConfigurationResponse(token)

	case onvif.PTZGetConfigurationOptions:
		b = onvif.PTZGetConfigurationOptionsResponse()

	case onvif.PTZGetCompatibleConfigurations:
		token := onvif.FindTagValue(b, "ProfileToken")
		b = onvif.PTZGetCompatibleConfigurationsResponse(token)

	case onvif.PTZContinuousMove:
		token := onvif.FindTagValue(b, "ProfileToken")
		pan, err := onvif.FindTagFloatAttribute(b, "PanTilt", "x")
		if err != nil {
			writeONVIFFault(w, err)
			return
		}
		tilt, err := onvif.FindTagFloatAttribute(b, "PanTilt", "y")
		if err != nil {
			writeONVIFFault(w, err)
			return
		}
		zoom, err := onvif.FindTagFloatAttribute(b, "Zoom", "x")
		if err != nil {
			writeONVIFFault(w, err)
			return
		}
		timeout, err := onvif.ParsePTZTimeout(onvif.FindTagValue(b, "Timeout"))
		if err != nil {
			writeONVIFFault(w, err)
			return
		}
		if timeout == 0 {
			timeout = time.Second
		}
		if err = streams.PTZContinuousMove(token, core.PTZMove{Pan: pan, Tilt: tilt, Zoom: zoom, Timeout: timeout}); err != nil {
			writeONVIFFault(w, err)
			return
		}
		b = onvif.PTZContinuousMoveResponse()

	case onvif.PTZStop:
		token := onvif.FindTagValue(b, "ProfileToken")
		if err = streams.PTZStop(token); err != nil {
			writeONVIFFault(w, err)
			return
		}
		b = onvif.PTZStopResponse()

	case onvif.PTZGetStatus:
		token := onvif.FindTagValue(b, "ProfileToken")
		info, err := streams.PTZGetInfo(token)
		if err != nil {
			writeONVIFFault(w, err)
			return
		}
		b = onvif.PTZGetStatusResponse(info.Status)

	default:
		http.Error(w, "unsupported operation", http.StatusBadRequest)
		log.Warn().Msgf("[onvif] unsupported operation: %s", operation)
		log.Debug().Msgf("[onvif] unsupported request:\n%s", b)
		return
	}

	log.Trace().Msgf("[onvif] server response:\n%s", b)
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	if _, err = w.Write(b); err != nil {
		log.Error().Err(err).Caller().Send()
	}
}

func writeONVIFFault(w http.ResponseWriter, err error) {
	log.Warn().Err(err).Msg("[onvif] PTZ request failed")
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(onvif.FaultResponse(err.Error()))
}

func apiOnvif(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	var items []*api.Source

	if src == "" {
		devices, err := onvif.DiscoveryStreamingDevices()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, device := range devices {
			u, err := url.Parse(device.URL)
			if err != nil {
				log.Warn().Str("url", device.URL).Msg("[onvif] broken")
				continue
			}
			if u.Scheme != "http" {
				log.Warn().Str("url", device.URL).Msg("[onvif] unsupported")
				continue
			}
			u.Scheme = "onvif"
			u.User = url.UserPassword("user", "pass")
			if u.Path == onvif.PathDevice {
				u.Path = ""
			}
			items = append(items, &api.Source{Name: u.Host, URL: u.String(), Info: device.Name + " " + device.Hardware})
		}
	} else {
		client, err := onvif.NewClient(src)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if l := log.Trace(); l.Enabled() {
			b, _ := client.MediaRequest(onvif.MediaGetProfiles)
			l.Msgf("[onvif] src=%s profiles:\n%s", src, b)
		}
		name, err := client.GetName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tokens, err := client.GetProfilesTokens()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for i, token := range tokens {
			items = append(items, &api.Source{Name: name + " stream" + strconv.Itoa(i), URL: src + "?subtype=" + token})
		}
		if len(tokens) > 0 && client.HasSnapshots() {
			items = append(items, &api.Source{Name: name + " snapshot", URL: src + "?subtype=" + tokens[0] + "&snapshot"})
		}
	}
	api.ResponseSources(w, items)
}
