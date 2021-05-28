package pamidicontrol

// https://www.freedesktop.org/wiki/Software/PulseAudio/Documentation/Developer/Clients/DBus/#controlapi

import (
	"github.com/godbus/dbus"
	"github.com/rs/zerolog/log"
	"github.com/sqp/pulseaudio"
)

type PAClient struct {
	*pulseaudio.Client

	playbackStreamsByName map[string][]dbus.ObjectPath
	recordStreamsByName   map[string][]dbus.ObjectPath
	sourcesByName         map[string][]dbus.ObjectPath
	sinksByName           map[string][]dbus.ObjectPath

	midiClient *MidiClient
}

func NewPAClient(c *pulseaudio.Client) *PAClient {
	client := &PAClient{
		Client:                c,
		playbackStreamsByName: make(map[string][]dbus.ObjectPath, 0),
		recordStreamsByName:   make(map[string][]dbus.ObjectPath, 0),
		sourcesByName:         make(map[string][]dbus.ObjectPath, 0),
		sinksByName:           make(map[string][]dbus.ObjectPath, 0),
	}

	return client
}

func (c *PAClient) NewPlaybackStream(path dbus.ObjectPath) {
	stream := c.Stream(path)
	props, err := stream.MapString("PropertyList")
	if err != nil {
		log.Warn().Err(err)
		return
	}

	if applicationName, ok := props["application.name"]; ok {
		log.Info().Msgf("stream %s appeared", applicationName)
		log.Info().Msgf("props: %v", props)
		c.recordUpdated(applicationName, true)
	}

	c.RefreshStreams()
}

func (c *PAClient) PlaybackStreamRemoved(path dbus.ObjectPath) {
	log.Info().Msgf("%v", path)
	for name, paths := range c.playbackStreamsByName {
		for _, p := range paths {
			if p == path && len(paths) == 1 {
				c.recordUpdated(name, false)
				c.muteUpdated(name, false)
			}
		}
	}
	c.RefreshStreams()
}

func (c *PAClient) recordUpdated(name string, present bool) {
	if key, ok := recordKeys[name]; ok {
		if present {
			log.Info().Msgf("%v on", key)
			c.midiClient.LedOn(key)
		} else {
			log.Info().Msgf("%v off", key)
			c.midiClient.LedOff(key)
		}
	}
}

// DeviceVolumeUpdated is called when the volume has changed on a device.
func (c *PAClient) DeviceVolumeUpdated(path dbus.ObjectPath, values []uint32) {
	log.Info().Msgf("one: device volume updated: %v %v", path, values)
}

var muteKeys = map[string]uint8{
	"spotify":  1,
	"Firefox":  4,
	"Chromium": 7,
	"Focusrite Scarlett 2i2 2nd Gen Analog Stereo": 10,
	"Webcam C270 Mono":    13,
	"Jabra Link 380 Mono": 16,
}

var recordKeys = map[string]uint8{
	"spotify":  3,
	"Firefox":  6,
	"Chromium": 9,
	"Focusrite Scarlett 2i2 2nd Gen Analog Stereo": 12,
	"Webcam C270 Mono":    15,
	"Jabra Link 380 Mono": 18,
}

func (c *PAClient) muteUpdated(name string, mute bool) {
	if key, ok := muteKeys[name]; ok {
		if mute {
			c.midiClient.LedOn(key)
		} else {
			c.midiClient.LedOff(key)
		}
	}
}

func (c *PAClient) DeviceMuteUpdated(path dbus.ObjectPath, mute bool) {
	device := c.Device(path)
	props, err := device.MapString("PropertyList")
	if err != nil {
		log.Warn().Err(err)
		return
	}

	if deviceDescription, ok := props["device.description"]; ok {
		log.Info().Msgf("device %s mute updated: %v", deviceDescription, mute)
		c.muteUpdated(deviceDescription, mute)
	}
}

func (c *PAClient) StreamMuteUpdated(path dbus.ObjectPath, mute bool) {
	stream := c.Stream(path)
	props, err := stream.MapString("PropertyList")
	if err != nil {
		log.Warn().Err(err)
		return
	}

	if applicationName, ok := props["application.name"]; ok {
		log.Info().Msgf("stream %s mute updated: %v", applicationName, mute)
		c.muteUpdated(applicationName, mute)
	}
}

func (c *PAClient) RefreshStreams() error {
	playbackStreamsByName := make(map[string][]dbus.ObjectPath, 0)
	recordStreamsByName := make(map[string][]dbus.ObjectPath, 0)
	sinksByName := make(map[string][]dbus.ObjectPath, 0)
	sourcesByName := make(map[string][]dbus.ObjectPath, 0)

	streams, err := c.Core().ListPath("PlaybackStreams")
	if err != nil {
		return err
	}

	for _, streamPath := range streams {
		stream := c.Stream(streamPath)
		props, err := stream.MapString("PropertyList")
		if err != nil {
			return err
		}

		if applicationName, ok := props["application.name"]; ok {
			if _, ok := playbackStreamsByName[applicationName]; ok {
				playbackStreamsByName[applicationName] = append(playbackStreamsByName[applicationName], streamPath)
			} else {
				playbackStreamsByName[applicationName] = []dbus.ObjectPath{streamPath}
			}
		}
	}

	streams, err = c.Core().ListPath("RecordStreams")
	if err != nil {
		return err
	}

	for _, streamPath := range streams {
		stream := c.Stream(streamPath)
		props, err := stream.MapString("PropertyList")
		if err != nil {
			return err
		}

		log.Info().Msgf("%v", props)
		if applicationName, ok := props["application.name"]; ok {
			if _, ok := recordStreamsByName[applicationName]; ok {
				recordStreamsByName[applicationName] = append(recordStreamsByName[applicationName], streamPath)
			} else {
				recordStreamsByName[applicationName] = []dbus.ObjectPath{streamPath}
			}
		}
	}

	sinks, err := c.Core().ListPath("Sinks")
	if err != nil {
		return err
	}
	for _, sinkPath := range sinks {
		device := c.Device(sinkPath)
		props, err := device.MapString("PropertyList")
		if err != nil {
			panic(err)
		}

		if deviceDescription, ok := props["device.description"]; ok {
			if _, ok := sinksByName[deviceDescription]; ok {
				sinksByName[deviceDescription] = append(sinksByName[deviceDescription], sinkPath)
			} else {
				sinksByName[deviceDescription] = []dbus.ObjectPath{sinkPath}
			}
		}
	}

	sources, err := c.Core().ListPath("Sources")
	if err != nil {
		return err
	}
	for _, sourcePath := range sources {
		device := c.Device(sourcePath)
		props, err := device.MapString("PropertyList")
		if err != nil {
			panic(err)
		}

		if deviceDescription, ok := props["device.description"]; ok {
			if _, ok := sourcesByName[deviceDescription]; ok {
				sourcesByName[deviceDescription] = append(sourcesByName[deviceDescription], sourcePath)
			} else {
				sourcesByName[deviceDescription] = []dbus.ObjectPath{sourcePath}
			}
		}
	}

	c.playbackStreamsByName = playbackStreamsByName
	c.recordStreamsByName = recordStreamsByName
	c.sinksByName = sinksByName
	c.sourcesByName = sourcesByName
	return nil
}

func (c *PAClient) ProcessVolumeAction(action PulseAudioAction, volume float32) error {
	pa100perc := 65535
	newVol := uint32(volume * float32(pa100perc))

	objs := make([]*pulseaudio.Object, 0)

	if action.TargetType == Sink {
		if sinkPaths, ok := c.sinksByName[action.TargetName]; ok {
			for _, sinkPath := range sinkPaths {
				objs = append(objs, c.Device(sinkPath))
			}
		}
	}

	if action.TargetType == Source {
		if sourcePaths, ok := c.sourcesByName[action.TargetName]; ok {
			for _, sourcePath := range sourcePaths {
				log.Info().Msgf("set volume %v: %v", sourcePath, newVol)
				objs = append(objs, c.Device(sourcePath))
			}
		}
	}

	if action.TargetType == PlaybackStream {
		if streamPaths, ok := c.playbackStreamsByName[action.TargetName]; ok {
			for _, streamPath := range streamPaths {
				objs = append(objs, c.Stream(streamPath))
			}
		}
	}

	if action.TargetType == RecordStream {
		if streamPaths, ok := c.recordStreamsByName[action.TargetName]; ok {
			for _, streamPath := range streamPaths {
				objs = append(objs, c.Stream(streamPath))
			}
		}
	}

	if len(objs) > 0 {
		for _, obj := range objs {
			vol := make([]uint32, 0)
			if channels, err := obj.ListUint32("Channels"); err != nil {
				log.Info().Msgf("couldn't get channels: %v", err)
			} else {
				for range channels {
					vol = append(vol, newVol)
				}
			}

			err := obj.Set("Volume", vol)
			if err != nil {
				return err
			}
		}
	} else {
		var paType string
		switch action.TargetType {
		case Sink:
			paType = "sink"
		case Source:
			paType = "source"
		case PlaybackStream:
			paType = "playback stream"
		case RecordStream:
			paType = "record stream"
		}

		log.Warn().Msgf("Could not find %s by name [%s] to set its volume", paType, action.TargetName)
	}
	return nil
}

func (c *PAClient) ProcessMuteAction(action PulseAudioAction) error {
	objs := make([]*pulseaudio.Object, 0)

	if action.TargetType == Sink {
		if sinkPaths, ok := c.sinksByName[action.TargetName]; ok {
			for _, sinkPath := range sinkPaths {
				objs = append(objs, c.Device(sinkPath))
			}
		}
	}

	if action.TargetType == Source {
		if sourcePaths, ok := c.sourcesByName[action.TargetName]; ok {
			for _, sourcePath := range sourcePaths {
				objs = append(objs, c.Device(sourcePath))
			}
		}
	}

	if action.TargetType == PlaybackStream {
		if streamPaths, ok := c.playbackStreamsByName[action.TargetName]; ok {
			for _, streamPath := range streamPaths {
				objs = append(objs, c.Stream(streamPath))
			}
		}
	}

	if action.TargetType == RecordStream {
		if streamPaths, ok := c.recordStreamsByName[action.TargetName]; ok {
			for _, streamPath := range streamPaths {
				objs = append(objs, c.Stream(streamPath))
			}
		}
	}

	if len(objs) > 0 {
		for _, obj := range objs {
			mute, _ := obj.Bool("Mute")
			if err := obj.Set("Mute", !mute); err != nil {
				return err
			}
		}
	} else {
		var paType string
		switch action.TargetType {
		case Sink:
			paType = "sink"
		case Source:
			paType = "source"
		case PlaybackStream:
			paType = "playback stream"
		case RecordStream:
			paType = "record stream"
		}

		log.Warn().Msgf("Could not find %s by name [%s] to set it mute", paType, action.TargetName)
	}
	return nil
}

func (c *PAClient) UpdateRecordingLeds() {
	for name, _ := range recordKeys {
		c.recordUpdated(name, false)
	}
	for name, _ := range c.playbackStreamsByName {
		c.recordUpdated(name, true)
	}
	for name, _ := range c.recordStreamsByName {
		c.recordUpdated(name, true)
	}
	for name, _ := range c.sinksByName {
		c.recordUpdated(name, true)
	}
	for name, _ := range c.sourcesByName {
		c.recordUpdated(name, true)
	}
}
