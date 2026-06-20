package main

import (
	"context"
	"flag"
	"log"

	// "os"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/meow/termcall/internal/capture"
	"github.com/meow/termcall/internal/rtc"
	"github.com/meow/termcall/internal/signaling"
	"github.com/meow/termcall/internal/tui"
)

func main() {
	serverURL := flag.String("server", "ws://localhost:8080/ws", "Signaling server URL")
	roomID := flag.String("room", "test", "Room ID")
	username := flag.String("username", "user", "Username")
	flag.Parse()

	// ...
	// Setup context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}
	defer f.Close()

	log.Printf("Connecting to %s, Room: %s", *serverURL, *roomID)

	sigClient, err := signaling.NewClient(ctx, *serverURL, *roomID, *username)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer sigClient.Close()

	mesh := rtc.NewMeshManager(sigClient)

	cam := capture.NewCamera(15)
	camChan, err := cam.Start(ctx, 40, 20)
	if err != nil {
		log.Fatalf("Failed to start camera: %v", err)
	}

	mic := capture.NewMicrophone()
	micTrack, err := mic.Start(ctx)
	if err != nil {
		log.Printf("Warning: failed to start microphone: %v", err)
	} else {
		mesh.SetLocalAudioTrack(micTrack)
		defer mic.Close()
	}

	// Setup Bubble Tea
	p := tea.NewProgram(tui.NewAppModel(mesh, cam, mic), tea.WithAltScreen())

	mesh.OnPeerJoin = func(peerID, username string) {
		// handle joined event if needed
	}

	mesh.OnPeerLeave = func(peerID string) {
		// handle leave event if needed
	}

	mesh.OnFrame = func(peerID string, frame string) {
		p.Send(tui.PeerFrameMsg{PeerID: peerID, Frame: frame})
	}

	// Capture frames from camera
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-camChan:
				if !ok {
					return
				}
				p.Send(tui.LocalFrameMsg{Frame: frame})
			}
		}
	}()

	mesh.Start()

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	log.Println("Shutting down...")
	mesh.Stop()
}
