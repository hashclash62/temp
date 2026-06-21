# TermCall Architecture & Progress Report

This document is intended to provide detailed context for AI coding agents about the current implementation logic, routing, and WebRTC network traversal of the TermCall application.

## 1. Core Architecture
TermCall is a terminal-based video calling application written in Go.
- **Topology:** Currently implemented as a **Full Mesh Network** (`internal/rtc/MeshManager`). Every peer maintains a direct `PeerConnection` with every other peer in the room.
- **Signaling Server:** A centralized WebSocket server (`cmd/termcall-server`) running on an AWS EC2 instance handles room state and forwards SDP Offers, Answers, and ICE Candidates.
- **WebRTC Stack:** Built entirely on the [Pion WebRTC](https://github.com/pion/webrtc) stack.

## 2. Media Routing & Implementation Logic

### Video (ASCII Text)
- **Capture:** Webcam captured via `pion/mediadevices`.
- **Processing:** Converted into ASCII characters locally using an image-to-ASCII renderer before transmission.
- **Routing:** Instead of using standard RTP video tracks, the ASCII string is broadcasted over a **WebRTC DataChannel** (Label: `video-ascii`).
- **Configuration:** The DataChannel is configured as **Unordered** and **Unreliable** (MaxRetransmits = 0) to mimic real-time UDP video behavior (we don't care about dropped frames, only the latest frame).

### Audio (Opus)
- **Capture:** Microphone captured via `pion/mediadevices`, encoding raw audio into Opus frames natively.
- **Routing:** Sent over standard WebRTC RTP Audio Tracks (`webrtc.TrackLocal`). 
- **Playback:** Handled by `internal/playback/player_cgo.go` using `malgo` for device playback and `pion/opus` for decoding.
- **Mute Functionality:** Implemented via `webrtc.RTPSender.ReplaceTrack(nil)`. This gracefully stops outbound packet transmission without tearing down the underlying `PeerConnection`.

## 3. Network Traversal (AWS VPS Configuration)
Network traversal (NAT punching) was heavily debugged and optimized for hostile ISP environments (e.g., Symmetric NATs like Jio mobile networks).

### STUN (Public IP Discovery)
- We rely on `stun:stun.l.google.com:19302` as the primary STUN provider. This guarantees reliable `srflx` candidate generation for 85% of users. 

### TURN (Relay Server)
Symmetric NATs prevent direct STUN P2P connections. For these users, media *must* be relayed through our TURN server on the AWS VPS.
- **Custom TURN Server:** We use `pion/turn` integrated directly into our `termcall-server` binary.
- **The TCP Encapsulation Bypass:** Many ISPs explicitly throttle or drop UDP packets. To guarantee connectivity across strict firewalls, we enabled **TURN-over-TCP** and bound the TURN server to **TCP Port 443** (the standard HTTPS port).
- **ICE URL:** `turn:13.127.137.230:443?transport=tcp`
- **Result:** WebRTC traffic is encapsulated inside TCP and disguised as standard HTTPS traffic, bypassing almost all ISP filters.

### AWS Server Ports Checklist
If deploying a new AWS EC2 server, the following ports MUST be open in both **AWS Security Groups** and the **OS Firewall (ufw/iptables)**:
- `TCP 8080`: WebSocket Signaling.
- `TCP 443`: TURN-over-TCP listener (Critical for NAT traversal bypass).
- `UDP 443`: TURN-over-UDP listener.
- `UDP 50000 - 50050`: TURN Relay port range (Used to forward the actual media packets).

## 4. Scalability Limits & Future Directions
Because this is a Full Mesh topology:
- Each peer uploads their stream `N-1` times. 
- Total upload bandwidth is ~300 kbps per connection (~40kbps Opus + ~260kbps ASCII frames).
- **Practical Limit:** 10-15 peers per room.
- **Server Cost:** If all users are behind Symmetric NATs (worst-case scenario), the AWS TURN server relays $N \times (N-1)$ streams. A 10-person room costs ~27 Mbps of continuous server bandwidth.

### Future Scale Paths
1. **1-on-1 Mode:** If restricted to 2-person rooms, STUN handles 85% of calls completely P2P. Server bandwidth drops to near-zero, scaling to 50k+ concurrent users on a cheap VPS.
2. **SFU Integration:** For 50+ user webinars, `MeshManager` must be replaced with an SFU (Selective Forwarding Unit) like LiveKit, meaning clients only upload 1 stream to the server.
