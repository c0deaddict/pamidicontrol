package pamidicontrol

import (
	"github.com/rs/zerolog/log"
	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/reader"
	"gitlab.com/gomidi/midi/writer"
	driver "gitlab.com/gomidi/portmididrv"
)

type MidiClient struct {
	PAClient       *PAClient
	MidiActions    []MidiAction
	InputMidiName  string
	OutputMidiName string
	wr             *writer.Writer
}

func (c *MidiClient) ListDevices() ([]string, []string, error) {
	drv, err := driver.New()
	if err != nil {
		panic(err)
	}

	// make sure to close all open ports at the end
	defer drv.Close()

	ins, err := drv.Ins()
	if err != nil {
		return nil, nil, err
	}

	outs, err := drv.Outs()
	if err != nil {
		return nil, nil, err
	}

	inNames := make([]string, 0)
	outNames := make([]string, 0)

	for _, port := range ins {
		inNames = append(inNames, port.String())
	}

	for _, port := range outs {
		outNames = append(outNames, port.String())
	}

	return inNames, outNames, nil
}

func (c *MidiClient) LedOn(key uint8) {
	writer.NoteOn(c.wr, key, 127)
}

func (c *MidiClient) LedOff(key uint8) {
	// NOTE: strangely enough, writer.NoteOff(..) doesn't work?
	c.wr.Write(writer.Channel(c.wr).NoteOff(key))
}

func (c *MidiClient) Run(initDone chan<- struct{}) {
	drv, err := driver.New()
	if err != nil {
		panic(err)
	}

	// make sure to close all open ports at the end
	defer drv.Close()

	ins, err := drv.Ins()
	if err != nil {
		panic(err)
	}

	outs, err := drv.Outs()
	if err != nil {
		panic(err)
	}

	var in midi.In
	var out midi.Out

	for _, port := range ins {
		log.Info().Msgf("Found input midi device: %s", port.String())
		if port.String() == c.InputMidiName {
			in = port
		}
	}

	for _, port := range outs {
		log.Info().Msgf("Found output midi device: %s", port.String())
		if port.String() == c.OutputMidiName {
			out = port
		}
	}

	if err := in.Open(); err != nil {
		panic(err)
	}

	if err := out.Open(); err != nil {
		panic(err)
	}

	defer in.Close()
	defer out.Close()

	c.wr = writer.New(out)

	rd := reader.New(
		reader.NoLogger(),
		reader.Each(func(pos *reader.Position, msg midi.Message) {
			log.Info().Msgf("%v", msg)
			switch midiMessage := msg.(type) {
			case channel.ControlChange:
				for _, action := range c.MidiActions {
					if action.ActionType != ControlChange {
						continue
					}

					if action.Channel != midiMessage.Channel() {
						continue
					}

					if action.Controller != midiMessage.Controller() {
						continue
					}

					perc := float32(midiMessage.Value()) / float32(action.MaxInputValue)

					if action.Action.ActionType == VolumeChange {
						if err := c.PAClient.ProcessVolumeAction(action.Action, perc); err != nil {
							log.Error().Err(err)
						}
					}
				}
				log.Info().Msgf("Saw ControlChange input on Channel %d, Controller %d, with value %d", midiMessage.Channel(), midiMessage.Controller(), midiMessage.Value())

			case channel.NoteOn:
				// c.wr.Write(writer.Channel(c.wr).NoteOff(midiMessage.Key()))
				for _, action := range c.MidiActions {
					if action.ActionType != NoteOn {
						continue
					}

					if action.Channel != midiMessage.Channel() {
						continue
					}

					if action.Controller != midiMessage.Key() {
						continue
					}

					if action.Action.ActionType == VolumeChange {
						if err := c.PAClient.ProcessMuteAction(action.Action); err != nil {
							log.Error().Err(err)
						}
					}
				}
				log.Info().Msgf("Saw MuteOn input on Channel %d, Key %d, with value %d", midiMessage.Channel(), midiMessage.Key(), midiMessage.Velocity())
			}
		}),
	)

	if err != nil {
		panic(err)
	}

	close(initDone)
	rd.ListenTo(in)
}
