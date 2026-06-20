package capture

import (
	"context"
	"fmt"

	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/opus"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/webrtc/v4"
)

type Microphone struct {
	track mediadevices.Track
}

func NewMicrophone() *Microphone {
	return &Microphone{}
}

func (m *Microphone) Start(ctx context.Context) (webrtc.TrackLocal, error) {
	opusParams, err := opus.NewParams()
	if err != nil {
		return nil, fmt.Errorf("opus params: %w", err)
	}

	stream, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Audio: func(c *mediadevices.MediaTrackConstraints) {},
		Codec: mediadevices.NewCodecSelector(
			mediadevices.WithAudioEncoders(&opusParams),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("getting user media (mic): %w", err)
	}

	tracks := stream.GetAudioTracks()
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks available")
	}

	m.track = tracks[0]
	return m.track.(webrtc.TrackLocal), nil
}

func (m *Microphone) Close() {
	if m.track != nil {
		m.track.Close()
	}
}
