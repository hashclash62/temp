# TermCall — Terminal-Based Group Video Call TUI

> A peer-to-peer, terminal-native group video calling application built in Go with Pion WebRTC.

---

## Phase 1 — Scope and Non-Goals

### What does this do?

TermCall is a terminal-based group video/audio call application. A user creates a **room**, receives a unique room ID, and shares it with others. Peers join by providing a username and room ID. Video from each peer's webcam is captured, converted to ASCII art on the client side, and streamed to all other peers via WebRTC data channels. Audio is sent as a standard WebRTC Opus audio track. The TUI renders a Zoom-like grid layout showing all participants' ASCII video feeds, with keybindings to toggle video/audio. Rooms persist as long as at least one peer is connected (or until explicit cleanup), and peers can leave and rejoin freely.

### Who/what calls it?

- **Human CLI user**: Runs a single Go binary (`termcall`) in their terminal
- The binary is the client — it connects to a lightweight signaling server over WebSockets, then establishes direct peer-to-peer WebRTC connections with other participants
- The signaling server is a separate self-hosted Go binary (`termcall-server`)

### Explicit non-goals

- **No browser client** — this is terminal-only
- **No video codec encoding/decoding** — we send ASCII text frames over data channels, not encoded video tracks
- **No recording or playback** — live calls only
- **No end-to-end encryption beyond WebRTC's built-in DTLS-SRTP** — we rely on the transport encryption WebRTC provides
- **No user accounts or persistent authentication** — users supply a username at join time; no passwords, no database
- **No chat/messaging feature** — this is a v1; text chat can be added later via data channels
- **No mobile or embedded targets** — desktop Linux/macOS/Windows only
- **No SFU/MCU** — mesh only for v1 (max 5 peers)

### What kind of project is this?

Two separate Go binaries:
1. **`termcall-server`** — A long-running service (signaling + TURN)
2. **`termcall`** — A CLI application (client TUI)

---

## Mesh vs. SFU: Can Mesh Work for 10–20 People?

### Short answer: No, not practically.

In a mesh topology, every participant must establish a direct connection to every other participant. For N participants:
- Each client sends N-1 outgoing streams and receives N-1 incoming streams
- Total connections in the network: N×(N-1)/2

| Participants | Connections per client | Total connections | Upload bandwidth (1 Mbps stream) |
|:---:|:---:|:---:|:---:|
| 3 | 2 | 3 | 2 Mbps |
| 5 | 4 | 10 | 4 Mbps |
| 10 | 9 | 45 | 9 Mbps |
| 20 | 19 | 190 | 19 Mbps |

### Why mesh breaks at scale

1. **Bandwidth explosion**: A user sending even a 1 Mbps stream needs 19 Mbps upload for 20 peers. Most home connections cannot sustain this.
2. **CPU overload**: Each client must encode once per peer and decode N-1 incoming streams simultaneously. This causes stuttering, overheating, and battery drain on laptops.
3. **NAT traversal complexity**: More connections = more chances that some peers can't establish direct connections and need TURN relay, which then concentrates load on your TURN server anyway.

### Why mesh works well for our case (≤5 peers)

However, for our **ASCII video** use case specifically, mesh is actually **more viable than typical video calls**:

- **ASCII frames are tiny**: A typical 80×24 terminal ASCII frame is ~2 KB. Even at 15 FPS, that's only ~30 KB/s per peer (~240 Kbps). For 5 peers, that's ~960 Kbps upload — completely manageable.
- **Audio is lightweight**: Opus audio at conversational quality is ~32-64 Kbps per peer.
- **Total per client for 5 peers**: ~(240 + 64) × 4 = ~1.2 Mbps — trivial for any modern internet connection.
- **No video encoding CPU cost**: We're sending text, not encoding H.264/VP8.

### Recommendation

| Peers | Topology | Reason |
|:---:|:---:|:---|
| **≤5** | **Mesh** ✅ | ASCII frames are tiny; total bandwidth ~1.2 Mbps; zero server media load |
| **6-10** | **Mesh might work** ⚠️ | ASCII keeps bandwidth low (~3 Mbps), but connection management becomes complex |
| **>10** | **SFU required** ❌ | Even with ASCII, 20 peers = 190 WebRTC connections; ICE/DTLS overhead dominates |

**For v1, we hard-cap at 5 peers with full mesh.** If you want to scale to 10-20 in the future, you'd need to add an SFU (Pion makes this straightforward — the signaling server would gain a media relay layer). The architecture below is designed so this migration path is clean.

---

## Phase 2 — Architecture Sketch

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        SELF-HOSTED SERVER                          │
│                                                                     │
│  ┌─────────────────────────┐    ┌────────────────────────────────┐  │
│  │   Signaling Server      │    │    STUN/TURN Server            │  │
│  │   (WebSocket)           │    │    (pion/turn)                  │  │
│  │                         │    │                                 │  │
│  │  • Room management      │    │  • STUN: NAT discovery         │  │
│  │  • Peer registry        │    │  • TURN: Relay fallback        │  │
│  │  • SDP/ICE relay        │    │  • Credential auth             │  │
│  │  • Join/leave events    │    │                                 │  │
│  └────────────┬────────────┘    └────────────────────────────────┘  │
│               │                                                     │
└───────────────┼─────────────────────────────────────────────────────┘
                │ WebSocket
                │
    ┌───────────┼──────────────────────────────────────────┐
    │           │           PEER MESH                       │
    │           ▼                                           │
    │   ┌──────────────┐         ┌──────────────┐          │
    │   │   Client A   │◄───────►│   Client B   │          │
    │   │              │  WebRTC │              │           │
    │   │ • Webcam     │  (P2P)  │ • Webcam     │          │
    │   │ • ASCII conv │         │ • ASCII conv │          │
    │   │ • Mic capture│         │ • Mic capture│          │
    │   │ • TUI render │         │ • TUI render │          │
    │   └──────┬───────┘         └──────┬───────┘          │
    │          │                        │                   │
    │          │      ┌──────────────┐  │                   │
    │          └─────►│   Client C   │◄─┘                   │
    │                 │              │                       │
    │                 │ • Webcam     │                       │
    │                 │ • ASCII conv │                       │
    │                 │ • Mic capture│                       │
    │                 │ • TUI render │                       │
    │                 └──────────────┘                       │
    └───────────────────────────────────────────────────────┘

    Data flow per peer:
    ┌──────────┐     ┌───────────┐     ┌───────────┐     ┌──────────┐
    │ Webcam   │────►│ ASCII     │────►│ DataChannel│────►│ Remote   │
    │ Capture  │     │ Converter │     │ (unreliable)│    │ TUI      │
    └──────────┘     └───────────┘     └───────────┘     └──────────┘

    ┌──────────┐     ┌───────────┐     ┌───────────┐
    │ Mic      │────►│ Opus      │────►│ Audio Track│────► Remote speakers
    │ Capture  │     │ Encode    │     │ (RTP)      │
    └──────────┘     └───────────┘     └───────────┘
```

### Servers You Need to Self-Host

| Server | Purpose | Load Profile | Technology |
|:---|:---|:---|:---|
| **Signaling Server** | Room management, SDP/ICE relay between peers | Very light — only WebSocket messages, no media | Go binary using `gorilla/websocket` or `nhooyr/websocket` |
| **STUN/TURN Server** | NAT traversal (STUN) and relay fallback (TURN) | Light for STUN (stateless); TURN only used when direct P2P fails | `pion/turn` library (embedded in signaling server, or separate) |

> **Key insight for minimizing server load**: In mesh mode, your server handles **zero media**. The signaling server only relays small JSON messages during connection setup. The TURN server only activates when peers can't connect directly (symmetric NAT, corporate firewalls). For most home users, STUN alone suffices and is stateless.

### Deployment Options

**Option A — Single binary (Recommended for simplicity)**
Embed the signaling server and STUN/TURN server into a single Go binary. The STUN/TURN listener runs on UDP ports alongside the WebSocket HTTP server. This minimizes operational overhead.

**Option B — Two separate binaries**
Run signaling and TURN as separate processes. Better for scaling independently, but overkill for ≤5 peers.

**We go with Option A for v1.**

### Module Layout

```
termcall/
├── cmd/
│   ├── termcall/              # Client CLI binary
│   │   └── main.go
│   └── termcall-server/       # Server binary (signaling + STUN/TURN)
│       └── main.go
├── internal/
│   ├── signaling/             # Signaling server: WebSocket, rooms, message routing
│   │   ├── server.go          # HTTP + WebSocket server setup
│   │   ├── room.go            # Room lifecycle, peer tracking
│   │   ├── message.go         # Signaling protocol message types
│   │   └── handler.go         # WebSocket message handlers
│   ├── turn/                  # STUN/TURN server setup and credential generation
│   │   └── server.go
│   ├── rtc/                   # WebRTC peer connection management (client-side)
│   │   ├── peer.go            # Single PeerConnection wrapper
│   │   ├── mesh.go            # Mesh manager: creates/destroys peer connections
│   │   ├── datachannel.go     # ASCII frame data channel send/receive
│   │   └── audio.go           # Audio track management (Opus)
│   ├── capture/               # Media capture (client-side)
│   │   ├── camera.go          # Webcam capture abstraction
│   │   ├── ascii.go           # Frame → ASCII conversion
│   │   └── microphone.go      # Microphone capture
│   ├── tui/                   # Terminal UI (client-side)
│   │   ├── app.go             # Bubble Tea main model
│   │   ├── layout.go          # Grid layout engine (Zoom-like grid)
│   │   ├── video_panel.go     # Single peer's ASCII video panel
│   │   ├── controls.go        # Status bar, keybinding display
│   │   └── styles.go          # Lipgloss styles
│   └── protocol/              # Shared types between client and server
│       └── messages.go        # Signaling message types (JSON schemas)
├── go.mod
├── go.sum
├── Makefile
├── DESIGN.md
└── README.md
```

### Package Responsibilities

| Package | Responsibility | Used by |
|:---|:---|:---|
| `cmd/termcall` | CLI entry point, flag parsing, wires everything together | — |
| `cmd/termcall-server` | Server entry point, config, starts signaling + TURN | — |
| `internal/signaling` | WebSocket server, room state, message routing | Server only |
| `internal/turn` | Configures and starts pion/turn STUN/TURN listeners | Server only |
| `internal/rtc` | Manages PeerConnections, data channels, audio tracks | Client only |
| `internal/capture` | Webcam/mic capture, ASCII conversion | Client only |
| `internal/tui` | Bubble Tea TUI, layout, rendering | Client only |
| `internal/protocol` | Shared message type definitions (JSON) | Both client + server |

### Dependency Direction (no cycles)

```
cmd/termcall-server
    └──► internal/signaling
    │        └──► internal/protocol
    └──► internal/turn

cmd/termcall
    └──► internal/tui
    │        └──► internal/rtc
    │        │        └──► internal/protocol
    │        └──► internal/capture
    └──► internal/rtc
    └──► internal/capture
    └──► internal/protocol
```

No cycles. `protocol` is a leaf dependency shared by both sides. Client packages never import server packages and vice versa.

### Interface Placement

Interfaces are defined at the **consumer** (Go convention):

| Interface | Defined in (consumer) | Implemented by |
|:---|:---|:---|
| `FrameSource` | `internal/rtc` | `internal/capture` (camera) |
| `AudioSource` | `internal/rtc` | `internal/capture` (microphone) |
| `SignalTransport` | `internal/rtc` | Client's WebSocket signaling connection |
| `PeerEventHandler` | `internal/tui` | `internal/rtc` (notifies TUI of peer join/leave/frames) |

---

## Phase 3 — Concurrency Model

### Model: Pipeline with goroutines + channels

This project has substantial concurrent work. Each pipeline stage runs in its own goroutine, communicating via channels:

**Client-side goroutine architecture:**

```
Main goroutine (Bubble Tea event loop)
│
├── Signaling goroutine
│   └── Reads/writes WebSocket messages
│   └── Feeds events to Bubble Tea via tea.Cmd
│
├── Camera capture goroutine
│   └── Reads webcam frames at ~15 FPS
│   └── Converts to ASCII
│   └── Sends ASCII frames to all data channels
│
├── Microphone capture goroutine
│   └── Reads PCM audio from mic
│   └── Encodes to Opus
│   └── Writes to audio tracks
│
├── Per-peer goroutine × (N-1)
│   └── Handles incoming data channel messages (ASCII frames)
│   └── Sends to TUI via channel for rendering
│   └── Handles incoming audio track → local playback
│
└── Audio playback goroutine
    └── Mixes incoming audio from all peers
    └── Writes to speaker output
```

**Server-side goroutine architecture:**

```
Main goroutine (HTTP server)
│
├── Per-connection goroutine × N (WebSocket read loop)
│   └── Reads messages, dispatches to room
│
├── Per-connection goroutine × N (WebSocket write loop)
│   └── Writes messages from room's broadcast channel
│
└── TURN server goroutine(s)
    └── Managed by pion/turn internally
```

### Shared Mutable State

| State | Protection | Rationale |
|:---|:---|:---|
| **Room peer list** (server) | `sync.RWMutex` | Reads (broadcast) far exceed writes (join/leave) |
| **Active rooms map** (server) | `sync.RWMutex` | Same pattern — many lookups, rare creates/deletes |
| **Peer connection map** (client) | `sync.Mutex` | Mutated on join/leave events; small critical section |
| **TUI model state** (client) | None needed — Bubble Tea's `Update` is single-threaded | All mutations go through `tea.Msg` |
| **Camera on/off toggle** (client) | `atomic.Bool` | Simple flag read by capture goroutine |
| **Mic on/off toggle** (client) | `atomic.Bool` | Same pattern |

### Context and Cancellation

- Every goroutine receives a `context.Context` derived from the application's root context
- Server: root context cancels on SIGINT/SIGTERM → graceful WebSocket close → room cleanup
- Client: root context cancels on user quit (`q` / `Ctrl+C`) → closes all peer connections → closes signaling WebSocket
- Per-peer contexts are derived from the room/mesh context and cancel individually when a peer leaves

---

## Phase 4 — Data & Interface Design

### Core Structs

```go
// ===== internal/protocol/messages.go =====
// Shared between client and server

type MessageType string

const (
    MsgJoinRoom      MessageType = "join_room"
    MsgLeaveRoom     MessageType = "leave_room"
    MsgPeerJoined    MessageType = "peer_joined"
    MsgPeerLeft      MessageType = "peer_left"
    MsgOffer         MessageType = "offer"
    MsgAnswer        MessageType = "answer"
    MsgICECandidate  MessageType = "ice_candidate"
    MsgRoomCreated   MessageType = "room_created"
    MsgError         MessageType = "error"
    MsgTURNCredentials MessageType = "turn_credentials"
)

type SignalingMessage struct {
    Type     MessageType     `json:"type"`
    RoomID   string          `json:"room_id,omitempty"`
    PeerID   string          `json:"peer_id,omitempty"`
    Username string          `json:"username,omitempty"`
    Payload  json.RawMessage `json:"payload,omitempty"`
}

type SDPPayload struct {
    SDP  string `json:"sdp"`
    Type string `json:"type"` // "offer" or "answer"
}

type ICECandidatePayload struct {
    Candidate     string `json:"candidate"`
    SDPMid        string `json:"sdp_mid"`
    SDPMLineIndex int    `json:"sdp_mline_index"`
}

type TURNCredentials struct {
    URLs       []string `json:"urls"`
    Username   string   `json:"username"`
    Credential string   `json:"credential"`
}

// ===== internal/signaling/room.go =====

type Peer struct {
    ID       string
    Username string
    Conn     *websocket.Conn
    Send     chan []byte   // Outbound message queue
}

type Room struct {
    ID        string
    Peers     map[string]*Peer  // PeerID → Peer
    mu        sync.RWMutex
    CreatedAt time.Time
    MaxPeers  int               // Hard cap: 5
}

// ===== internal/rtc/peer.go =====

type RemotePeer struct {
    PeerID     string
    Username   string
    PC         *webrtc.PeerConnection
    DataChan   *webrtc.DataChannel  // For ASCII frames
    AudioTrack *webrtc.TrackRemote  // Incoming audio
    LastFrame  []byte               // Latest ASCII frame for rendering
    mu         sync.Mutex
}

// ===== internal/rtc/mesh.go =====

type MeshManager struct {
    LocalPeerID string
    Peers       map[string]*RemotePeer
    mu          sync.Mutex
    
    // Outbound: our ASCII frames go here, mesh fans out to all peers
    localFrames <-chan []byte
    
    // Inbound: received frames from peers, consumed by TUI
    onFrame     func(peerID string, frame []byte)
    onPeerJoin  func(peerID, username string)
    onPeerLeave func(peerID string)
    
    signaler    SignalTransport
    iceServers  []webrtc.ICEServer
}

// ===== internal/capture/camera.go =====

type ASCIIFrame struct {
    Width    int      // Characters wide
    Height   int      // Characters tall
    Data     string   // The ASCII art string
    PeerID   string   // Who this frame belongs to
}

// ===== internal/tui/app.go =====

type Model struct {
    // Peer state
    localPeerID string
    localUser   string
    roomID      string
    peers       map[string]*PeerView  // Remote peers
    
    // Local media state
    videoOn     bool
    audioOn     bool
    
    // Layout
    width       int
    height      int
    
    // Dependencies
    mesh        *rtc.MeshManager
    camera      *capture.Camera
    mic         *capture.Microphone
}

type PeerView struct {
    PeerID   string
    Username string
    Frame    string   // Current ASCII frame to render
    AudioOn  bool     // Is this peer sending audio?
    VideoOn  bool     // Is this peer sending video?
}
```

### Key Interfaces

```go
// ===== Defined in internal/rtc (consumer) =====
// Implemented by capture package

type FrameSource interface {
    // Start begins producing frames. Frames are sent to the returned channel.
    // Cancel the context to stop capture.
    Start(ctx context.Context, width, height int) (<-chan []byte, error)
}

type AudioSource interface {
    // Start begins capturing audio. Returns a channel of Opus-encoded packets.
    Start(ctx context.Context) (<-chan []byte, error)
}

// Transport for signaling messages (implemented by WebSocket client)
type SignalTransport interface {
    Send(msg protocol.SignalingMessage) error
    Receive() <-chan protocol.SignalingMessage
    Close() error
}

// ===== Defined in internal/tui (consumer) =====
// Implemented by rtc package (via callbacks/channels)

// These are expressed as tea.Msg types rather than an interface,
// fitting the Bubble Tea pattern:

type PeerJoinedMsg struct {
    PeerID   string
    Username string
}

type PeerLeftMsg struct {
    PeerID string
}

type FrameReceivedMsg struct {
    PeerID string
    Frame  string
}

type PeerMediaStateMsg struct {
    PeerID  string
    VideoOn bool
    AudioOn bool
}
```

### ASCII Frame Data Channel Protocol

The data channel carries binary messages with a simple header:

```
[1 byte: message type] [payload]

Message types:
  0x01 = ASCII video frame
         Payload: raw UTF-8 string of the ASCII frame
  0x02 = Media state update  
         Payload: JSON {"video_on": bool, "audio_on": bool}
  0x03 = Ping/keepalive
         Payload: empty
```

This is intentionally minimal. The data channel is configured as **unordered + unreliable** (maxRetransmits=0) for video frames to minimize latency — dropped frames are simply replaced by the next one. Media state updates use a separate **reliable** data channel.

### Design Alternatives Considered

**Alternative 1: Video track (RTP) instead of data channel for ASCII frames**
- Pro: Standard WebRTC media path; could use simulcast
- Con: Massive overhead — encoding text as video pixels wastes bandwidth and CPU; requires codec setup (VP8/H264); adds encode/decode latency
- **Decision**: Data channel wins — ASCII text is inherently non-video data; data channels support it natively at a fraction of the cost

**Alternative 2: Single data channel (multiplexed) vs. separate channels per purpose**
- Pro: Simpler connection setup
- Con: Can't set different reliability/ordering per message type; video frames should be unreliable, state updates should be reliable
- **Decision**: Two data channels per peer — one unreliable for frames, one reliable for control messages

**Alternative 3: GoCV (OpenCV) vs. lighter camera capture (gocam / mediadevices)**
- GoCV: Battle-tested, powerful, but requires OpenCV C++ installation — heavy dependency for just frame capture
- pion/mediadevices: WebRTC-native, handles camera + codec, but may be more than needed
- gocam: Lightweight, pure capture, cross-platform
- **Decision**: Start with `pion/mediadevices` for camera capture — it integrates cleanly with the Pion stack we're already using, handles cross-platform concerns, and we only need raw frames (which we then convert to ASCII ourselves)

---

## Phase 5 — Dependencies, Risks, and Test Strategy

### Dependencies

| Dependency | Purpose | Alternatives Considered | Why this one |
|:---|:---|:---|:---|
| **`pion/webrtc`** | WebRTC peer connections, data channels, audio tracks | None viable — it's the only serious Go WebRTC library | De facto standard; pure Go; actively maintained |
| **`pion/turn`** | Self-hosted STUN/TURN server | Coturn (C), external hosted TURN | Pure Go, embeddable in our server binary, no external process; aligns with "all-Go" stack |
| **`charmbracelet/bubbletea`** | TUI framework (Model-View-Update) | `tview`, `tcell` (lower-level), `termbox` | Best developer experience; composable; built-in terminal management; `lipgloss` for styling |
| **`charmbracelet/lipgloss`** | TUI styling | Manual ANSI escape codes | Comes with Bubble Tea ecosystem; clean API |
| **`nhooyr.io/websocket`** | WebSocket client + server | `gorilla/websocket` | More modern API; context-aware; `gorilla/websocket` is in maintenance mode |
| **`pion/mediadevices`** | Camera + microphone capture | GoCV (heavy), `malgo` (audio-only), `blackjack/webcam` (Linux-only) | Cross-platform; WebRTC-native; handles both camera and mic; avoids OpenCV install |
| **`pion/opus`** | Opus audio encoding (pure Go) | `hraban/opus` (CGO) | Pure Go = no C dependencies; integrates with Pion stack |
| **stdlib `image`** | Image manipulation for ASCII conversion | GoCV image processing | Sufficient for grayscale + downscale; zero dependencies |
| **stdlib `encoding/json`** | Signaling protocol serialization | protobuf, msgpack | JSON is human-readable, debuggable; signaling messages are small and infrequent |

### Risk List

| # | Risk | Severity | Mitigation |
|:---|:---|:---:|:---|
| **1** | **Camera capture cross-platform issues** — `pion/mediadevices` may have rough edges on some OS/hardware combos | High | Spike camera capture first on all target platforms; fall back to GoCV if needed; provide `--no-video` flag |
| **2** | **ASCII rendering performance** — Converting frames to ASCII + rendering 4 peer panels at 15 FPS in a terminal may cause flickering/tearing | High | Use Bubble Tea's batched rendering; minimize redraws (diff-based updates); reduce FPS to 10 if needed; profile early |
| **3** | **Audio echo/feedback** — Without echo cancellation, speakers feed back into microphone | Medium | Recommend headphones in docs; investigate `pion/mediadevices` echo cancellation support; add push-to-talk as alternative |
| **4** | **NAT traversal failures** — Some networks block UDP entirely; TURN relay becomes critical path | Medium | Include TURN in the default server; test behind symmetric NAT; provide TCP TURN fallback |
| **5** | **Bubble Tea + concurrent WebRTC events** — Bridging Pion callbacks (which fire on internal goroutines) into Bubble Tea's single-threaded Update loop | Medium | Use `tea.Program.Send()` to inject messages from any goroutine; well-documented pattern in Bubble Tea |

### Prototype/Spike Order (address riskiest first)

1. **Spike 1**: Camera capture → ASCII conversion → render in terminal (proves the core visual concept)
2. **Spike 2**: Two peers connecting via Pion with data channel, exchanging ASCII frames (proves mesh + signaling)
3. **Spike 3**: Audio capture → Opus encode → audio track → playback (proves audio pipeline)
4. **Spike 4**: Full TUI with grid layout + keybindings (proves UX)
5. **Integration**: Wire all spikes together

### Test Strategy

**Unit Tests (table-driven, standard Go conventions):**
- `internal/protocol`: Message serialization/deserialization — table-driven tests with various message types
- `internal/signaling/room.go`: Room join/leave, max peer enforcement, concurrent access — use `testing.T.Parallel()`
- `internal/capture/ascii.go`: Image → ASCII conversion — table-driven with known input images and expected ASCII output
- `internal/tui/layout.go`: Grid layout calculation — table-driven with various peer counts and terminal sizes

**Integration Tests:**
- `internal/signaling`: Spin up in-memory WebSocket server, connect multiple test clients, verify SDP relay
- `internal/rtc`: Create two `MeshManager` instances with loopback signaling, verify data channel frame exchange
- End-to-end: Server + two clients exchanging ASCII frames (can use a test image instead of live webcam)

**Interfaces enabling testability:**
- `FrameSource` — mock with a static image in tests
- `AudioSource` — mock with silence or a test tone
- `SignalTransport` — mock with in-memory channels (no real WebSocket needed)
- `Room` methods — test directly without WebSocket layer

**What does NOT need automated tests (manual verification):**
- Actual webcam capture (hardware-dependent)
- TUI visual rendering (manual visual inspection)
- Audio quality (manual listening test)
- NAT traversal (requires real network topology testing)

---

## Signaling Protocol Specification

### Connection Flow

```
Client                          Server                          Other Clients
  │                               │                                   │
  │──── WS Connect ──────────────►│                                   │
  │                               │                                   │
  │──── join_room ───────────────►│                                   │
  │     {room_id, username}       │                                   │
  │                               │                                   │
  │◄─── turn_credentials ────────│                                   │
  │     {urls, username, cred}    │                                   │
  │                               │                                   │
  │◄─── peer_joined (×N) ────────│  (existing peers in room)         │
  │     {peer_id, username}       │                                   │
  │                               │──── peer_joined ─────────────────►│
  │                               │     {new peer's id, username}     │
  │                               │                                   │
  │──── offer ───────────────────►│                                   │
  │     {target_peer_id, sdp}     │──── offer ───────────────────────►│
  │                               │                                   │
  │                               │◄─── answer ──────────────────────│
  │◄─── answer ──────────────────│     {sdp}                         │
  │                               │                                   │
  │──── ice_candidate ──────────►│                                   │
  │     {target_peer_id, cand}    │──── ice_candidate ──────────────►│
  │                               │                                   │
  │  ← ─ ─ ─  P2P connection established  ─ ─ ─ →                   │
  │           (data channel + audio track)                            │
  │                               │                                   │
  │──── leave_room ─────────────►│                                   │
  │                               │──── peer_left ──────────────────►│
  │                               │                                   │
```

### Room Creation

When a client sends `join_room` with a `room_id` that doesn't exist, the server creates the room and responds with `room_created`. If the room exists, the client joins the existing room. The server generates a unique `peer_id` for each connection.

### TURN Credential Generation

On join, the server generates short-lived TURN credentials (HMAC-based, time-limited) and sends them to the client. This prevents unauthorized TURN usage without needing a database.

---

## TUI Layout Design

```
┌─────────────────────────────────────────────────────────────┐
│  TermCall — Room: abc-123-def          Peers: 3/5          │
├──────────────────────────┬──────────────────────────────────┤
│                          │                                  │
│   ┌──────────────────┐   │   ┌──────────────────────────┐   │
│   │  Alice (you)     │   │   │  Bob                     │   │
│   │                  │   │   │                          │   │
│   │   @@@@@@@@@@     │   │   │   @@@@@@@@@@             │   │
│   │  @          @    │   │   │  @          @            │   │
│   │  @ ◉      ◉ @   │   │   │  @ ◉      ◉ @           │   │
│   │  @    ▽     @    │   │   │  @    ▽     @            │   │
│   │  @  ╰────╯  @    │   │   │  @  ╰────╯  @           │   │
│   │  @          @    │   │   │  @          @            │   │
│   │   @@@@@@@@@@     │   │   │   @@@@@@@@@@             │   │
│   │                  │   │   │                          │   │
│   │  🎥 ON  🎤 ON   │   │   │  🎥 ON  🎤 ON           │   │
│   └──────────────────┘   │   └──────────────────────────┘   │
│                          │                                  │
├──────────────────────────┼──────────────────────────────────┤
│                          │                                  │
│   ┌──────────────────┐   │                                  │
│   │  Charlie          │   │                                  │
│   │                  │   │                                  │
│   │   @@@@@@@@@@     │   │                                  │
│   │  @          @    │   │                                  │
│   │  @ ◉      ◉ @   │   │                                  │
│   │  @    ▽     @    │   │                                  │
│   │  @  ╰────╯  @    │   │                                  │
│   │  @          @    │   │                                  │
│   │   @@@@@@@@@@     │   │                                  │
│   │                  │   │                                  │
│   │  🎥 ON  🎤 OFF  │   │                                  │
│   └──────────────────┘   │                                  │
│                          │                                  │
├─────────────────────────────────────────────────────────────┤
│  [v] Toggle Video  [m] Toggle Mic  [q] Leave Call          │
└─────────────────────────────────────────────────────────────┘
```

### Grid Layout Rules

| Peers | Layout | Panel Size |
|:---:|:---:|:---|
| 1 | 1×1 | Full width |
| 2 | 1×2 (side by side) | Half width each |
| 3-4 | 2×2 grid | Quarter each |
| 5 | 2×3 grid (bottom row has 1) | ~Third width |

### Keybindings

| Key | Action |
|:---|:---|
| `v` | Toggle local video on/off |
| `m` | Toggle local microphone on/off |
| `q` / `Ctrl+C` | Leave call and quit |
| `?` | Show/hide help overlay |

---

## Configuration

### Server Config (flags or env vars)

```
--listen-addr       HTTP listen address (default: ":8080")
--turn-addr         TURN/STUN listen address (default: ":3478")
--turn-public-ip    Public IP for TURN relay (required for TURN)
--turn-realm        TURN realm (default: "termcall")
--turn-secret       Shared secret for TURN credential generation
--max-room-peers    Max peers per room (default: 5)
--room-timeout      Timeout to garbage-collect empty rooms (default: "5m")
```

### Client Config (flags)

```
--server            Signaling server URL (e.g., "ws://myserver:8080/ws")
--room              Room ID to join (or "new" to create)
--username          Display name
--fps               ASCII frame rate (default: 15)
--width             ASCII frame width in chars (default: auto from terminal)
--no-video          Disable camera
--no-audio          Disable microphone
```

---

## Summary

| Component | Technology | Handles |
|:---|:---|:---|
| **Signaling Server** | Go + nhooyr/websocket | Room management, SDP/ICE relay |
| **STUN/TURN Server** | Go + pion/turn | NAT traversal, relay fallback |
| **Client WebRTC** | Go + pion/webrtc | Peer connections, data channels, audio |
| **Camera Capture** | pion/mediadevices | Cross-platform webcam access |
| **ASCII Conversion** | stdlib image + custom | Frame → ASCII art |
| **Audio Capture** | pion/mediadevices + pion/opus | Microphone → Opus |
| **TUI** | Bubble Tea + Lipgloss | Terminal rendering, keybindings |
| **Signaling Protocol** | JSON over WebSocket | Peer discovery, SDP exchange |

**Total self-hosted infrastructure**: One server binary (signaling + STUN/TURN). That's it.

**Server load**: Near-zero during calls. The server only handles WebSocket signaling messages (a few KB at connection time). All media flows peer-to-peer. TURN relay is only activated when direct P2P fails.

---

> **Does this match what you want, or should we revisit any phase?**
