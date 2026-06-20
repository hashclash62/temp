package rtc

import (
	"sync"

	"github.com/pion/webrtc/v4"
)

type RemotePeer struct {
	PeerID     string
	Username   string
	PC         *webrtc.PeerConnection
	DataChan   *webrtc.DataChannel // For ASCII frames
	ControlChan *webrtc.DataChannel // For reliable control messages
	AudioTrack *webrtc.TrackRemote // Incoming audio
	LastFrame  string              // Latest ASCII frame for rendering
	mu         sync.RWMutex
}

func (rp *RemotePeer) UpdateFrame(frame string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.LastFrame = frame
}

func (rp *RemotePeer) GetFrame() string {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.LastFrame
}

func (rp *RemotePeer) Close() {
	if rp.PC != nil {
		rp.PC.Close()
	}
}
