package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/meow/termcall/internal/capture"
	"github.com/meow/termcall/internal/rtc"
)

type AppModel struct {
	mesh      *rtc.MeshManager
	camera    *capture.Camera
	mic       *capture.Microphone
	camOn     bool
	micOn     bool

	width  int
	height int

	peers       []string
	peerFrames  map[string]string
	peerNames   map[string]string

	localFrame string
}

func NewAppModel(mesh *rtc.MeshManager, camera *capture.Camera, mic *capture.Microphone) *AppModel {
	return &AppModel{
		mesh:       mesh,
		camera:     camera,
		mic:        mic,
		camOn:      true,
		micOn:      true,
		peerFrames: make(map[string]string),
		peerNames:  make(map[string]string),
	}
}

// TickMsg triggers a UI re-render
type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// PeerFrameMsg represents a new ASCII frame from a peer
type PeerFrameMsg struct {
	PeerID string
	Frame  string
}

// LocalFrameMsg represents a local camera frame
type LocalFrameMsg struct {
	Frame []byte
}

func (m *AppModel) Init() tea.Cmd {
	return tickCmd()
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
			m.camOn = !m.camOn
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("m"))):
			m.micOn = !m.micOn
			// TODO: mute/unmute actual track
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case TickMsg:
		return m, tickCmd()
	case PeerFrameMsg:
		if _, ok := m.peerNames[msg.PeerID]; !ok {
			m.peerNames[msg.PeerID] = "Peer " + msg.PeerID
			m.peers = append(m.peers, msg.PeerID)
		}
		m.peerFrames[msg.PeerID] = msg.Frame
		return m, nil
	case LocalFrameMsg:
		m.localFrame = string(msg.Frame)
		if m.camOn {
			m.mesh.BroadcastFrame(msg.Frame)
		} else {
			m.localFrame = "Camera Off"
			m.mesh.BroadcastFrame([]byte("Camera Off"))
		}
		return m, nil
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m *AppModel) View() string {
	var views []string

	// Title Bar
	header := titleStyle.Render("TermCall (WebRTC CLI)")
	controls := fmt.Sprintf("[V]ideo: %v  [M]ic: %v  [Q]uit", m.camOn, m.micOn)
	views = append(views, lipgloss.JoinHorizontal(lipgloss.Left, header, "  ", controls))

	// Video Grid
	// Very simple row-based layout for now
	var grid []string

	// Local video
	localBox := boxStyle.Render("You\n\n" + m.localFrame)
	grid = append(grid, localBox)

	// Remote videos
	for _, p := range m.peers {
		name := m.peerNames[p]
		frame := m.peerFrames[p]
		if frame == "" {
			frame = "Waiting for video..."
		}
		peerBox := boxStyle.Render(name + "\n\n" + frame)
		grid = append(grid, peerBox)
	}

	// Join them in chunks of 2 horizontally
	var rows []string
	for i := 0; i < len(grid); i += 2 {
		if i+1 < len(grid) {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, grid[i], " ", grid[i+1]))
		} else {
			rows = append(rows, grid[i])
		}
	}

	views = append(views, lipgloss.JoinVertical(lipgloss.Top, rows...))

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}
