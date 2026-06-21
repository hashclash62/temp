package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/meow/termcall/internal/rtc"
	"github.com/meow/termcall/internal/signaling"
)

// Minimal test: connect two peers and check if they can establish a WebRTC connection.
func main() {
	serverURL := "ws://localhost:9090/ws"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	log.SetFlags(log.Ltime | log.Lmicroseconds)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect Peer A
	sigA, err := signaling.NewClient(ctx, serverURL, "diag", "PeerA")
	if err != nil {
		log.Fatalf("PeerA: failed to connect: %v", err)
	}
	defer sigA.Close()

	meshA := rtc.NewMeshManager(sigA)
	meshA.OnPeerJoin = func(peerID, username string) {
		log.Printf("PeerA: OnPeerJoin(%s, %s)", peerID, username)
	}
	meshA.OnFrame = func(peerID, frame string) {
		log.Printf("PeerA: received frame from %s (len=%d)", peerID, len(frame))
	}
	meshA.Start()

	// Wait a moment before connecting Peer B
	time.Sleep(500 * time.Millisecond)

	// Connect Peer B
	sigB, err := signaling.NewClient(ctx, serverURL, "diag", "PeerB")
	if err != nil {
		log.Fatalf("PeerB: failed to connect: %v", err)
	}
	defer sigB.Close()

	meshB := rtc.NewMeshManager(sigB)
	meshB.OnPeerJoin = func(peerID, username string) {
		log.Printf("PeerB: OnPeerJoin(%s, %s)", peerID, username)
	}
	meshB.OnFrame = func(peerID, frame string) {
		log.Printf("PeerB: received frame from %s (len=%d)", peerID, len(frame))
	}
	meshB.Start()

	// Now wait and watch the logs to see what happens
	log.Println("Both peers connected. Watching ICE negotiation for 20 seconds...")

	// Check periodically
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		meshA.RLockPeers(func(peers map[string]*rtc.RemotePeer) {
			for id, p := range peers {
				if p.PC != nil {
					log.Printf("PeerA -> %s: PC state=%s, ICE=%s", id, p.PC.ConnectionState().String(), p.PC.ICEConnectionState().String())
					if p.DataChan != nil {
						log.Printf("PeerA -> %s: DataChannel state=%s", id, p.DataChan.ReadyState().String())
					}
				}
			}
		})
		meshB.RLockPeers(func(peers map[string]*rtc.RemotePeer) {
			for id, p := range peers {
				if p.PC != nil {
					log.Printf("PeerB -> %s: PC state=%s, ICE=%s", id, p.PC.ConnectionState().String(), p.PC.ICEConnectionState().String())
					if p.DataChan != nil {
						log.Printf("PeerB -> %s: DataChannel state=%s", id, p.DataChan.ReadyState().String())
					}
				}
			}
		})
	}

	// Try to send a test message
	meshA.BroadcastFrame([]byte("hello from A"))
	time.Sleep(1 * time.Second)
	meshB.BroadcastFrame([]byte("hello from B"))
	time.Sleep(1 * time.Second)

	fmt.Println("Test complete.")
	meshA.Stop()
	meshB.Stop()

	// Give a second to print final logs
	time.Sleep(500 * time.Millisecond)
}
