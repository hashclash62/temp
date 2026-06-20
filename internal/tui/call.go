package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/meow/termcall/internal/ascii"
	"github.com/meow/termcall/internal/capture"
	"github.com/meow/termcall/internal/rtc"
)

type CallModel struct {
	mesh   *rtc.MeshManager
	camera *capture.Camera
	mic    *capture.Microphone
	camOn  bool
	micOn  bool

	width  int
	height int

	peers      []string
	peerFrames map[string]string
	peerNames  map[string]string

	localFrame []byte
	renderer   *ascii.DefaultRenderer
}

func NewCallModel(mesh *rtc.MeshManager, camera *capture.Camera, mic *capture.Microphone) *CallModel {
	return &CallModel{
		mesh:       mesh,
		camera:     camera,
		mic:        mic,
		camOn:      true,
		micOn:      true,
		peerFrames: make(map[string]string),
		peerNames:  make(map[string]string),
		renderer:   ascii.NewDefaultRenderer(ascii.Config{}),
	}
}

func (m *CallModel) Init() tea.Cmd {
	return nil
}

func (m *CallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "v":
			m.camOn = !m.camOn
			if !m.camOn {
				m.mesh.BroadcastFrame([]byte("Camera Off"))
			}
			return m, nil
		case "m":
			m.micOn = !m.micOn
			// TODO: mute/unmute actual track
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case PeerJoinMsg:
		m.AddPeer(msg.PeerID, msg.Username)
		return m, nil
	case PeerLeaveMsg:
		m.RemovePeer(msg.PeerID)
		return m, nil
	case PeerFrameMsg:
		if _, ok := m.peerNames[msg.PeerID]; !ok {
			// Name will be handled via OnPeerJoin normally, but fallback here
			m.peerNames[msg.PeerID] = "Peer " + msg.PeerID
			m.peers = append(m.peers, msg.PeerID)
		}
		m.peerFrames[msg.PeerID] = msg.Frame
		return m, nil
	case LocalFrameMsg:
		// Convert the raw image to ASCII at the exact size of the grid cell
		totalPeers := len(m.peers) + 1
		titleH := 1
		controlH := 3 // approx
		availH := m.height - titleH - controlH
		if availH < 0 {
			availH = 0
		}
		_, _, _, innerW, innerH := computeGrid(totalPeers, m.width, availH)

		asciiStr := m.renderer.Convert(msg.RawImage, innerW, innerH)
		m.localFrame = []byte(asciiStr)

		if m.camOn {
			m.mesh.BroadcastFrame(m.localFrame)
		}
		return m, nil
	}
	return m, nil
}

func (m *CallModel) View() string {
	theme := GetCurrentTheme()

	// If terminal size is not yet set, return empty
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// 1. Title bar (height: 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(theme.TitleFg).Background(theme.TitleBg).Padding(0, 1).Render(" TermCall ")
	titleBar := lipgloss.NewStyle().Width(m.width).Background(theme.TitleBg).Render(title)

	// 2. Control bar (height: 3 approx)
	controlBar := renderControls(m.width, m.camOn, m.micOn, theme)
	controlH := lipgloss.Height(controlBar)

	// 3. Grid area (remaining height)
	availH := m.height - lipgloss.Height(titleBar) - controlH
	if availH < 0 {
		availH = 0
	}

	// Total elements = Local + remote peers
	totalPeers := len(m.peers) + 1
	cols, boxW, boxH, innerW, innerH := computeGrid(totalPeers, m.width, availH)

	// Render cells
	var cells []string

	// Local cell
	localFrameStr := string(m.localFrame)
	if !m.camOn {
		localFrameStr = "Camera Off"
	}
	cells = append(cells, renderCell("You (Local)", localFrameStr, boxW, boxH, innerW, innerH, theme))

	// Remote cells
	for _, p := range m.peers {
		name := m.peerNames[p]
		frame := m.peerFrames[p]
		if frame == "" {
			frame = "Waiting for video..."
		}
		cells = append(cells, renderCell(name, frame, boxW, boxH, innerW, innerH, theme))
	}

	// Group into rows
	var rows []string
	for i := 0; i < len(cells); i += cols {
		end := i + cols
		if end > len(cells) {
			end = len(cells)
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Top, cells[i:end]...)
		rows = append(rows, rowStr)
	}

	gridStr := lipgloss.JoinVertical(lipgloss.Center, rows...)
	gridArea := lipgloss.Place(m.width, availH, lipgloss.Center, lipgloss.Center, gridStr)

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, gridArea, controlBar)
}

func renderCell(name, frame string, boxW, boxH, maxInnerW, maxInnerH int, theme Theme) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderColor).
		Padding(0, 1).
		Align(lipgloss.Center)

	// Truncate frame to prevent layout breaks from remote peers sending larger frames
	lines := strings.Split(frame, "\n")
	if len(lines) > maxInnerH {
		lines = lines[:maxInnerH]
	}
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) > maxInnerW {
			lines[i] = string(runes[:maxInnerW])
		}
	}
	safeFrame := strings.Join(lines, "\n")

	nameLabel := lipgloss.NewStyle().Bold(true).Foreground(theme.PeerLabelFg).Render(name)
	content := lipgloss.JoinVertical(lipgloss.Center, nameLabel, "", safeFrame)

	cellBlock := boxStyle.Render(content)
	return lipgloss.Place(boxW, boxH, lipgloss.Center, lipgloss.Center, cellBlock)
}

// AddPeer adds a peer's name to the list
func (m *CallModel) AddPeer(peerID, username string) {
	if _, ok := m.peerNames[peerID]; !ok {
		m.peers = append(m.peers, peerID)
	}
	m.peerNames[peerID] = username
}

// RemovePeer removes a peer from the list
func (m *CallModel) RemovePeer(peerID string) {
	for i, p := range m.peers {
		if p == peerID {
			m.peers = append(m.peers[:i], m.peers[i+1:]...)
			break
		}
	}
	delete(m.peerNames, peerID)
	delete(m.peerFrames, peerID)
}
