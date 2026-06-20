package turn

import (
	"fmt"
	"log"
	"net"

	"github.com/pion/turn/v4"
)

type Server struct {
	turnServer *turn.Server
}

// Config for TURN server
type Config struct {
	PublicIP string
	Port     int
	Realm    string
	Auth     turn.AuthHandler
}

// NewServer creates a new STUN/TURN server
func NewServer(cfg Config) (*Server, error) {
	udpListener, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to create TURN UDP listener: %w", err)
	}

	log.Printf("Starting STUN/TURN server on UDP %d (Public IP: %s)", cfg.Port, cfg.PublicIP)

	s, err := turn.NewServer(turn.ServerConfig{
		Realm:       cfg.Realm,
		AuthHandler: cfg.Auth,
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP(cfg.PublicIP),
					Address:      "0.0.0.0",
				},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create turn server: %w", err)
	}

	return &Server{turnServer: s}, nil
}

func (s *Server) Close() error {
	return s.turnServer.Close()
}
