package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net"
	"os"

	"github.com/meow/termcall/internal/signaling"
	"github.com/meow/termcall/internal/turn"
	turnPkg "github.com/pion/turn/v4"
)

func main() {
	port := flag.String("port", "8080", "HTTP listen port")
	turnPort := flag.Int("turn-port", 3478, "TURN server UDP port")
	turnIP := flag.String("turn-ip", "127.0.0.1", "TURN server public IP")
	flag.Parse()

	// Start STUN/TURN server
	_, err := turn.NewServer(turn.Config{
		PublicIP: *turnIP,
		Port:     *turnPort,
		Realm:    "termcall.local",
		Auth: func(username, realm string, srcAddr net.Addr) ([]byte, bool) {
			// Accept any username with password "termcall" for now
			return turnPkg.GenerateAuthKey(username, realm, "termcall"), true
		},
	})
	if err != nil {
		log.Printf("Failed to start TURN server: %v", err)
		// We can still continue with just STUN/Signaling
	}

	sigServer := signaling.NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", sigServer.HandleWebSocket)

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Starting TermCall signaling server on %s", addr)
	
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}
