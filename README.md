# TermCall

A fully-featured WebRTC Video Calling App that runs directly inside your terminal! Built with Go, Pion, and Bubbletea.

## Prerequisites

- **Go 1.20+** must be installed.
- **CGO** must be enabled (required for camera/microphone access via `mediadevices`).

### macOS

You need to have Xcode command line tools installed for CGO:
```bash
xcode-select --install
```

### Windows

Building on Windows requires a GCC compiler toolchain for CGO. The easiest way is to use MSYS2 or TDM-GCC:
1. Download and install [MSYS2](https://www.msys2.org/).
2. Open MSYS2 UCRT64 terminal and run:
   ```bash
   pacman -S mingw-w64-ucrt-x86_64-gcc
   ```
3. Add the MSYS2 `ucrt64/bin` folder to your system PATH.

## Installation & Running

1. Clone the repository and navigate into it.
2. Download dependencies:
   ```bash
   go mod tidy
   ```
3. Run the application:
   ```bash
   go run ./cmd/termcall
   ```

### Command Line Arguments

If you want to skip the join form and jump straight into a room:
```bash
go run ./cmd/termcall -room "my-secret-room" -username "Alice"
```

## Usage
- **[V]** Toggle Video
- **[M]** Toggle Microphone
- **[S]** Toggle Nerd Stats Overlay
- **[N]** Cycle Render Modes (ASCII -> Color256 -> HalfBlock)
- **[T]** Cycle UI Themes (Ayu, Catppuccin, Midnight, Daylight)
- **[Q]** Quit
