//go:build !cgo
// +build !cgo

package playback



type Player struct {
}

// NewPlayer initializes an audio player stub for non-CGO builds.
func NewPlayer() (*Player, error) {
	return &Player{}, nil
}

// WriteOpus safely ignores the incoming audio payload.
func (p *Player) WriteOpus(data []byte) error {
	return nil
}

// Close is a no-op.
func (p *Player) Close() {
}
