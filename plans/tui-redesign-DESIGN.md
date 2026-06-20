# TUI Redesign — Design Document

## Phase 1 — Scope and Non-Goals

### What this does
Redesigns the terminal UI for the TermCall WebRTC CLI client. The new TUI will be
minimal but polished, with a dynamic video grid, a bottom control bar, an initial
join-form when flags are missing, theming support, and a fully decoupled ASCII
rendering pipeline so the conversion algorithm can be swapped without touching
any other package.

### Who calls it
A human CLI user running the `termcall` binary in a macOS terminal.

### Non-goals
- Audio playback/rendering through the terminal (audio is WebRTC-only, no TUI concern).
- Implementing more than 2 color themes now (but the system must make adding more trivial).
- Chat/text messaging UI (future feature).
- Mobile/Windows terminal support (macOS only for now).

### Kind of project
This is a feature redesign within an existing CLI application. No new binaries or
services are being created. The changes are confined to `internal/tui/`,
`internal/ascii/` (new), and minor wiring in `cmd/termcall/main.go`.

---

## Phase 2 — Architecture Sketch

### Current state and problems

The current code has several architectural issues this redesign solves:

1. **ASCII conversion is locked inside `internal/capture/`** — the `imageToASCII`
   function lives in `capture/ascii.go` and is called directly by `Camera.Start()`.
   You can't change the character set, add colors, or experiment with the conversion
   without editing the capture pipeline.

2. **Portrait orientation** — `Camera.Start(ctx, 40, 20)` produces a 40-wide × 20-tall
   frame. Webcams capture in landscape (640×480 = 4:3). The current fixed dimensions
   don't respect the source aspect ratio and produce a squished portrait result.

3. **No join form** — if `--room` or `--username` is missing, the app uses hard-coded
   defaults. Users should be prompted.

4. **Rigid grid** — the `View()` function hard-codes 2 columns and does no size
   calculation.

5. **No theming** — colors are hard-coded inline.

### Proposed package layout

```
internal/
├── ascii/              ← NEW: decoupled ASCII renderer
│   ├── renderer.go     ← Renderer interface + default implementation
│   └── renderer_test.go
├── capture/
│   ├── camera.go       ← MODIFY: returns raw image.Image, no ASCII conversion
│   ├── ascii.go        ← DELETE: logic moves to internal/ascii/
│   └── microphone.go   ← unchanged
├── tui/
│   ├── app.go          ← REWRITE: orchestrates screens (join → call)
│   ├── join.go         ← NEW: huh-based join form screen
│   ├── call.go         ← NEW: main call screen (grid + controls)
│   ├── grid.go         ← NEW: dynamic video grid layout engine
│   ├── theme.go        ← NEW: theme definitions and switching
│   └── controls.go     ← NEW: bottom control bar widget
├── rtc/                ← unchanged
├── protocol/           ← unchanged
├── signaling/          ← unchanged
└── turn/               ← unchanged
```

### Dependency direction

```
cmd/termcall/main.go
  ├── internal/tui       (app, join, call)
  ├── internal/capture   (camera, microphone)
  ├── internal/ascii     (renderer)
  ├── internal/rtc       (mesh manager)
  └── internal/signaling (client)

internal/tui
  ├── internal/ascii     (renderer interface — for converting + rendering frames)
  ├── internal/rtc       (mesh manager — for peer state, broadcasting)
  └── internal/capture   (camera, mic — for toggling devices)

internal/ascii
  └── (stdlib only — image, strings, fmt)

internal/capture
  └── (no dependency on ascii — raw frames only)
```

No cycles. `ascii` depends on nothing internal. `capture` no longer calls `ascii`.
The TUI calls the renderer to convert raw frames before display/broadcast.

### Where do interfaces live?

| Interface | Defined at (consumer) | Reason |
|---|---|---|
| `ascii.Renderer` | `internal/ascii/renderer.go` | Small shared abstraction — both TUI and tests consume it; only one package owns the concept of "ASCII conversion". Exception to consumer-side rule is justified because the *purpose* of the package is the interface. |
| `capture.FrameSource` | `internal/capture/camera.go` | Already exists. Consumer is `cmd/termcall`. |

---

## Phase 3 — Concurrency Model

No changes to the existing concurrency model. The architecture remains:

- **One goroutine per camera frame reader** — reads `image.Image` from the camera,
  converts via `ascii.Renderer`, sends `[]byte` to Bubble Tea via `p.Send()`.
- **Bubble Tea owns the main loop** — all state updates go through the MVU cycle.
  No direct mutation from goroutines.
- **Shared state protection** — `MeshManager` already uses `sync.RWMutex`. The TUI
  model is single-threaded (Bubble Tea guarantees sequential `Update` calls).

The only new concurrent concern is that `Camera.Start` will now return
`<-chan image.Image` instead of `<-chan []byte`, and the frame-reading goroutine
in `main.go` will call `renderer.Convert(img, w, h)` before sending to Bubble Tea.
This keeps the renderer call on a goroutine (not blocking the UI) and keeps the
model single-threaded.

---

## Phase 4 — Data & Interface Design

### Core new interface: `ascii.Renderer`

```go
package ascii

import "image"

// Renderer converts a raw image frame to an ASCII string representation.
// Implementations can vary the character set, add ANSI color codes, use
// different scaling algorithms, etc.
type Renderer interface {
    // Convert transforms an image into an ASCII string of the given
    // terminal dimensions (width in columns, height in rows).
    Convert(img image.Image, width, height int) string
}

// Config holds tunable parameters for a renderer.
type Config struct {
    Gradient  string // character set from darkest to lightest
    Invert    bool   // flip luminance mapping
    ColorMode string // "none", "ansi256", "truecolor" (future)
}
```

**Default implementation:** `DefaultRenderer` — same nearest-neighbor + luminance
logic as today, but extracted and configurable via `Config`.

### Camera change: raw frames

```go
// Camera.Start signature change:
func (c *Camera) Start(ctx context.Context) (<-chan image.Image, error)
```

Width/height are no longer Camera's concern — the TUI decides dimensions
based on terminal size. The camera just produces raw `image.Image` frames.

### Theme struct

```go
package tui

type Theme struct {
    Name           string
    TitleFg        lipgloss.Color
    TitleBg        lipgloss.Color
    ControlBarBg   lipgloss.Color
    ControlBarFg   lipgloss.Color
    ActiveBtnFg    lipgloss.Color
    ActiveBtnBg    lipgloss.Color
    InactiveBtnFg  lipgloss.Color
    InactiveBtnBg  lipgloss.Color
    BorderColor    lipgloss.Color
    PeerLabelFg    lipgloss.Color
    StatusFg       lipgloss.Color
}
```

Two built-in themes: **"midnight"** (dark, purple/blue accents) and **"daylight"**
(light background, muted tones).

### TUI state machine

The `AppModel` will manage two screens via a `screen` enum:

```go
type screen int
const (
    screenJoin screen = iota
    screenCall
)
```

- `screenJoin` — shown when `--room` or `--username` was not provided. Uses
  `charmbracelet/huh` to render a form with two text inputs.
- `screenCall` — the main video call view (grid + controls).

### Grid layout algorithm

```
func computeGrid(peerCount, termWidth, termHeight int) (cols, cellW, cellH int)
```

Logic:
- 1 peer: 1 column, full width
- 2 peers: 2 columns, half width each
- 3-4 peers: 2 columns, 2 rows
- 5-6 peers: 3 columns, 2 rows

Cell height respects terminal character aspect ratio (chars are ~2:1 tall:wide),
so `cellH = cellW / 2` to produce landscape-looking ASCII frames.

### Landscape fix

The portrait problem happens because `Camera.Start(ctx, 40, 20)` passes
arbitrary dimensions that don't match the camera's 4:3 aspect ratio. The fix:

1. Camera returns raw `image.Image` (640×480 native).
2. The grid engine computes cell dimensions based on terminal size.
3. The renderer receives `(cellW, cellH)` and scales the native image to fit,
   preserving landscape orientation. Since terminal characters are approximately
   twice as tall as wide, `cellH` is already computed as `cellW / 2`, which
   maps naturally to the camera's landscape aspect ratio.

### Bottom control bar

```
╭──────────────────────────────────────────────────────╮
│  [V] Video: ON   [M] Mic: ON   [T] Theme   [Q] Quit │
╰──────────────────────────────────────────────────────╯
```

Fixed to the bottom of the terminal. Controls are highlighted/dimmed based on
their current on/off state. The bar is always visible during the call screen.

### Alternative designs considered

**Alternative 1: Keep ASCII inside capture, pass a config**
- Pro: fewer packages, simpler import graph.
- Con: `capture` becomes coupled to rendering concerns (character sets, colors).
  Any experiment with rendering requires editing the capture pipeline. Violates
  single-responsibility. Rejected.

**Alternative 2: Use `tview` instead of Bubble Tea**
- Pro: built-in grid/flex layout primitives, less boilerplate for form inputs.
- Con: we already depend on Bubble Tea and the Charm ecosystem (lipgloss, bubbles).
  Switching would mean rewriting everything and introducing a second paradigm.
  `tview`'s imperative style would fight with our existing MVU architecture.
  Rejected — sticking with Bubble Tea and adding `huh` for forms.

**Alternative 3: Use `tcell` directly for pixel-level control of the grid**
- Pro: maximum control over rendering, could do sub-character-cell tricks.
- Con: massive implementation effort for basic layout. We'd be writing our own
  TUI framework. Rejected.

---

## Phase 5 — Dependencies, Risks, and Test Strategy

### Dependency choices

| Need | Chosen | Alternatives considered | Reasoning |
|---|---|---|---|
| TUI framework | **Bubble Tea** (keep) | tview, tcell | Already in use. MVU is a good fit for reactive video frames. Switching would be a full rewrite with no clear benefit. |
| Styling | **Lipgloss** (keep) | tcell styles, ANSI raw codes | Already in use. Excellent composability. |
| Form/dialog | **charmbracelet/huh** | bubbles/textinput (manual wiring), survey (archived) | `huh` is the Charm ecosystem's dedicated form library. Integrates natively with Bubble Tea. `survey` is archived/unmaintained. Manual `textinput` wiring would duplicate what `huh` already does well. |
| Pre-built components | **charmbracelet/bubbles** (keep) | — | Already a dependency. We'll use `key` bindings from it. |

### Risks

1. **ASCII frame rate vs. terminal rendering speed** — if the grid is large (5 peers)
   and the terminal is slow, the `View()` function could become a bottleneck because
   it re-renders the entire screen every tick. Mitigation: only re-render when a frame
   actually changes (compare frame hashes, or use a dirty flag per peer).

2. **`huh` integration with Bubble Tea** — `huh` supports being embedded in a Bubble
   Tea model, but the integration pattern requires wrapping it as a sub-model. If the
   API is awkward, we fall back to manual `textinput` components. Low risk — `huh` is
   designed for exactly this.

3. **Aspect ratio math** — terminal characters are not square. The exact ratio depends
   on the font. We assume 2:1 (height:width). If users have unusual fonts, the video
   will look distorted. Mitigation: expose a `--char-ratio` flag later if needed.

### Test strategy

- **`internal/ascii`**: table-driven unit tests. Feed known `image.Image` inputs
  (solid color, gradient, checkerboard) and assert the output string matches expected
  ASCII. Test that `Config.Gradient` actually changes output characters.
- **`internal/tui/grid.go`**: table-driven tests for `computeGrid()`. Input: peer
  count + terminal dimensions. Output: expected cols, cellW, cellH. Pure math, easy
  to test.
- **`internal/tui/theme.go`**: verify that `GetTheme("midnight")` and
  `GetTheme("daylight")` return valid, non-zero `Theme` structs.
- **Integration**: manual testing — run the binary without `--room` and `--username`,
  verify the form appears. Connect two clients and verify the grid resizes dynamically.

---

## Implementation Order (Spikes)

### Spike 1: Extract `internal/ascii` and decouple from `capture`
- Create `internal/ascii/renderer.go` with `Renderer` interface + `DefaultRenderer`.
- Modify `Camera.Start()` to return `<-chan image.Image`.
- Update `cmd/termcall/main.go` to create a renderer and convert frames before
  sending to Bubble Tea.
- Delete `internal/capture/ascii.go`.
- **Verify:** `go build ./cmd/termcall` compiles. Manual test: video still displays.

### Spike 2: Theme system
- Create `internal/tui/theme.go` with `Theme` struct, `midnight` and `daylight`
  themes, and a `GetTheme(name string)` function.
- Wire theme into `AppModel`. Add `t` keybinding to cycle themes.
- **Verify:** pressing `t` changes the UI colors.

### Spike 3: Join form
- Create `internal/tui/join.go` using `charmbracelet/huh`.
- Modify `cmd/termcall/main.go`: if `--room` or `--username` is empty, start the
  TUI in `screenJoin` mode. After form submit, transition to `screenCall`.
- **Verify:** run without flags → form appears → fill in → call starts.

### Spike 4: Dynamic grid + bottom controls
- Create `internal/tui/grid.go` with `computeGrid()`.
- Create `internal/tui/controls.go` for the bottom bar.
- Rewrite `internal/tui/call.go` (replaces old `app.go` View logic) to use the
  grid engine and render landscape-proportioned ASCII frames.
- **Verify:** connect 1-5 peers, grid adapts. Bottom bar always visible.

### Spike 5: Polish and cleanup
- Remove old `internal/tui/app.go` (replaced by new files).
- Ensure `--room` and `--username` flags still work (skip form if both provided).
- Final manual test across two MacBooks.
