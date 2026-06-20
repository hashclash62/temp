# TermCall - Progress Report

## Current Status
We have successfully built a functional, cross-platform terminal-based video calling application. The core feature of transmitting real-time webcam video as ASCII art over WebRTC data channels is working efficiently, alongside a polished and thematic Terminal User Interface (TUI).

## Completed Features

### 1. Signaling & WebRTC Mesh Architecture
- **WebSocket Signaling Server**: Implemented a standalone server (`cmd/termcall-server`) that manages room creation, peer discovery, and forwards SDP offers/answers and ICE candidates.
- **Embedded TURN Server**: The signaling server embeds a `pion/turn` server to enable NAT traversal for clients on restrictive networks.
- **Mesh Topology**: Implemented an N-way mesh connection (`internal/rtc/mesh.go`) where every client forms a peer-to-peer connection with every other client. Glare conditions (simultaneous offers) are automatically resolved using deterministic PeerID comparisons.

### 2. Video Capture & ASCII Rendering
- **Hardware Integration**: The camera feed is read directly using `pion/mediadevices`.
- **ASCII Conversion**: Raw `image.Image` frames are processed dynamically (`internal/ascii/renderer.go`).
- **Dynamic Resolution**: The TUI measures the terminal size and grid cell dimensions in real-time, enforcing a proper 4:3 landscape aspect ratio constraint, and passes the exact target dimensions to the ASCII renderer to prevent terminal wrapping or artifacting.

### 3. Terminal User Interface (TUI)
- **Framework**: Built natively with Bubble Tea (`charmbracelet/bubbletea`) and Lipgloss.
- **Join Form**: Uses `charmbracelet/huh` to provide an interactive setup form if the CLI flags (`--room`, `--username`) are omitted.
- **Dynamic Grid**: Video feeds from the local camera and remote peers are dynamically arranged into rows and columns depending on the number of participants.
- **Theme System**: Supports hot-swapping visual styles via the `[T]` key, currently featuring Midnight, Daylight, and Ayu themes.
- **Cross-Platform Fallbacks**: Conditionally drops CGO dependencies (like audio capture) if they are missing on the host OS, letting the app gracefully degrade into a text-video-only mode (crucial for frictionless Windows compilation).

## Outstanding Issues & Next Steps

### 1. Audio Playback (Microphone Module)
**Issue**: While the app captures and transmits Opus audio streams over WebRTC, the receiver currently drops incoming audio packets. There is no decoding or playback engine implemented.
**Next Step**: Implement Opus decoding (via `hraban/opus`) and audio playback (via `malgo` or `oto`) and wire them to the remote WebRTC audio tracks.

### 2. Audio Control UI
**Issue**: The `[M]` (Mic) button in the UI toggles visual state, but does not yet physically mute the WebRTC audio track.
**Next Step**: Wire the TUI control bar directly to the WebRTC track states.

### 3. Mesh Scalability (Future)
**Issue**: The current mesh topology works flawlessly for 2-5 peers but network overhead scales quadratically ($O(n^2)$).
**Next Step**: To support 10-20 peers, we must investigate an SFU (Selective Forwarding Unit) architecture to centralize bandwidth usage.
