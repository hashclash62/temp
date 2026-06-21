package rtc

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/meow/termcall/internal/playback"
	"github.com/meow/termcall/internal/protocol"
	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
)

type MeshManager struct {
	LocalPeerID string
	Peers       map[string]*RemotePeer
	mu          sync.RWMutex

	signaler   SignalTransport
	webrtcAPI  *webrtc.API
	iceServers []webrtc.ICEServer

	localAudioTrack webrtc.TrackLocal

	// Callbacks
	OnFrame     func(peerID string, frame string)
	OnPeerJoin  func(peerID, username string)
	OnPeerLeave func(peerID string)

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMeshManager(signaler SignalTransport) *MeshManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare MediaEngine (will add Opus later)
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Fatalf("Failed to register codecs: %v", err)
	}

	s := webrtc.SettingEngine{}
	s.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(s))

	return &MeshManager{
		Peers:     make(map[string]*RemotePeer),
		signaler:  signaler,
		webrtcAPI: api,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (m *MeshManager) Start() {
	go m.readSignalingLoop()
}

func (m *MeshManager) Stop() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.Peers {
		p.Close()
	}
}

func (m *MeshManager) SetICEServers(servers []webrtc.ICEServer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iceServers = servers
}

func (m *MeshManager) SetLocalAudioTrack(track webrtc.TrackLocal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localAudioTrack = track
}

func (m *MeshManager) BroadcastFrame(frame []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.Peers {
		if p.DataChan != nil && p.DataChan.ReadyState() == webrtc.DataChannelStateOpen {
			err := p.DataChan.SendText(string(frame))
			if err != nil {
				// log.Printf("Failed to send frame to %s: %v", p.PeerID, err)
			}
		}
	}
}

func (m *MeshManager) readSignalingLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case msg, ok := <-m.signaler.Receive():
			if !ok {
				return
			}
			m.handleSignalingMessage(msg)
		}
	}
}

func (m *MeshManager) handleSignalingMessage(msg protocol.SignalingMessage) {
	switch msg.Type {
	case protocol.MsgRoomCreated:
		m.mu.Lock()
		m.LocalPeerID = msg.PeerID
		m.mu.Unlock()
		log.Printf("Joined room %s as %s", msg.RoomID, msg.PeerID)

	case protocol.MsgTURNCredentials:
		var creds protocol.TURNCredentials
		if err := json.Unmarshal(msg.Payload, &creds); err == nil {
			m.SetICEServers([]webrtc.ICEServer{
				{
					URLs:       creds.URLs,
					Username:   creds.Username,
					Credential: creds.Credential,
				},
			})
		}

	case protocol.MsgPeerJoined:
		m.handlePeerJoined(msg)

	case protocol.MsgPeerLeft:
		m.handlePeerLeft(msg.PeerID)

	case protocol.MsgOffer:
		m.handleOffer(msg)

	case protocol.MsgAnswer:
		m.handleAnswer(msg)

	case protocol.MsgICECandidate:
		m.handleICECandidate(msg)
	}
}

func (m *MeshManager) createPeerConnection(peerID string) (*webrtc.PeerConnection, error) {
	m.mu.RLock()
	iceServers := m.iceServers
	m.mu.RUnlock()

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	pc, err := m.webrtcAPI.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	localTrack := m.localAudioTrack
	m.mu.RUnlock()

	if localTrack != nil {
		if _, err := pc.AddTrack(localTrack); err != nil {
			log.Printf("Failed to add local audio track: %v", err)
		}
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Received %s track from %s", track.Kind().String(), peerID)
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			player, err := playback.NewPlayer()
			if err != nil {
				log.Printf("Failed to create audio player for %s: %v", peerID, err)
				return
			}

			go func() {
				defer player.Close()
				for {
					packet, _, err := track.ReadRTP()
					if err != nil {
						return
					}
					if err := player.WriteOpus(packet.Payload); err != nil {
						// Silence decode errors, as packet loss can corrupt frames
					}
				}
			}()
		}
	})

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		
		// pion v4 changes ICECandidate struct.
		// We'll safely marshal it.
		candJSON := c.ToJSON()

		payload, _ := json.Marshal(protocol.ICECandidatePayload{
			Candidate:     candJSON.Candidate,
			SDPMid:        candJSON.SDPMid,
			SDPMLineIndex: candJSON.SDPMLineIndex,
			UsernameFragment: candJSON.UsernameFragment,
		})

		m.signaler.Send(protocol.SignalingMessage{
			Type:    protocol.MsgICECandidate,
			PeerID:  peerID,
			Payload: payload,
		})
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("Peer %s state changed: %s", peerID, s.String())
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			m.handlePeerLeft(peerID)
		}
	})

	return pc, nil
}

func (m *MeshManager) handlePeerJoined(msg protocol.SignalingMessage) {
	peerID := msg.PeerID

	m.mu.RLock()
	_, exists := m.Peers[peerID]
	m.mu.RUnlock()
	if exists {
		return // Already have this peer
	}

	pc, err := m.createPeerConnection(peerID)
	if err != nil {
		log.Printf("Failed to create PC for %s: %v", peerID, err)
		return
	}

	peer := &RemotePeer{
		PeerID:   peerID,
		Username: msg.Username,
		PC:       pc,
	}

	m.mu.Lock()
	m.Peers[peerID] = peer
	m.mu.Unlock()

	if m.OnPeerJoin != nil {
		m.OnPeerJoin(peerID, peer.Username)
	}

	// Only initiate offer if LocalPeerID < peerID to avoid glare
	if m.LocalPeerID > peerID {
		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			if dc.Label() == "video-ascii" {
				peer.DataChan = dc
				dc.OnMessage(func(dMsg webrtc.DataChannelMessage) {
					if dMsg.IsString {
						peer.UpdateFrame(string(dMsg.Data))
						if m.OnFrame != nil {
							m.OnFrame(peerID, string(dMsg.Data))
						}
					}
				})
			}
		})
		return
	}

	// Create unordered, unreliable datachannel for video frames
	maxRetransmits := uint16(0)
	ordered := false
	dc, err := pc.CreateDataChannel("video-ascii", &webrtc.DataChannelInit{
		MaxRetransmits: &maxRetransmits,
		Ordered:        &ordered,
	})
	if err != nil {
		log.Printf("Failed to create data channel: %v", err)
		return
	}
	peer.DataChan = dc

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			peer.UpdateFrame(string(msg.Data))
			if m.OnFrame != nil {
				m.OnFrame(peerID, string(msg.Data))
			}
		}
	})

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Printf("Failed to create offer: %v", err)
		return
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	payload, _ := json.Marshal(protocol.SDPPayload{
		Type: "offer",
		SDP:  offer.SDP,
	})

	m.signaler.Send(protocol.SignalingMessage{
		Type:    protocol.MsgOffer,
		PeerID:  peerID,
		Payload: payload,
	})
}

func (m *MeshManager) handleOffer(msg protocol.SignalingMessage) {
	peerID := msg.PeerID

	m.mu.RLock()
	peer, exists := m.Peers[peerID]
	m.mu.RUnlock()

	if !exists {
		log.Printf("Received offer from unknown peer %s", peerID)
		return
	}

	var offerPayload protocol.SDPPayload
	if err := json.Unmarshal(msg.Payload, &offerPayload); err != nil {
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerPayload.SDP,
	}

	if err := peer.PC.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote desc: %v", err)
		return
	}

	answer, err := peer.PC.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		return
	}

	if err := peer.PC.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local desc: %v", err)
		return
	}

	payload, _ := json.Marshal(protocol.SDPPayload{
		Type: "answer",
		SDP:  answer.SDP,
	})

	m.signaler.Send(protocol.SignalingMessage{
		Type:    protocol.MsgAnswer,
		PeerID:  peerID,
		Payload: payload,
	})
}

func (m *MeshManager) handleAnswer(msg protocol.SignalingMessage) {
	m.mu.RLock()
	peer, ok := m.Peers[msg.PeerID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	var answerPayload protocol.SDPPayload
	if err := json.Unmarshal(msg.Payload, &answerPayload); err != nil {
		return
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerPayload.SDP,
	}

	if err := peer.PC.SetRemoteDescription(answer); err != nil {
		log.Printf("Failed to set remote desc: %v", err)
	}
}

func (m *MeshManager) handleICECandidate(msg protocol.SignalingMessage) {
	m.mu.RLock()
	peer, ok := m.Peers[msg.PeerID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	var candPayload protocol.ICECandidatePayload
	if err := json.Unmarshal(msg.Payload, &candPayload); err != nil {
		return
	}

	cand := webrtc.ICECandidateInit{
		Candidate:        candPayload.Candidate,
		SDPMid:           candPayload.SDPMid,
		SDPMLineIndex:    candPayload.SDPMLineIndex,
		UsernameFragment: candPayload.UsernameFragment,
	}

	if err := peer.PC.AddICECandidate(cand); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	}
}

func (m *MeshManager) handlePeerLeft(peerID string) {
	m.mu.Lock()
	peer, ok := m.Peers[peerID]
	if ok {
		delete(m.Peers, peerID)
	}
	m.mu.Unlock()

	if ok {
		peer.Close()
		if m.OnPeerLeave != nil {
			m.OnPeerLeave(peerID)
		}
	}
}
