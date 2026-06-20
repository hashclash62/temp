package main

import (
	"context"
	"flag"
	"log"
	"sync/atomic"

	// "os"
	"github.com/charmbracelet/bubbletea"
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

	skipForm := *roomID != "" && *username != ""

	var p atomic.Pointer[tea.Program]
	send := func(msg tea.Msg) {
		if prog := p.Load(); prog != nil {
			prog.Send(msg)
		}
	}

	// We wrap the WebRTC connection logic in a function so it can be called
	// after the user submits the form (if the form is shown).
	startCall := func(res tui.JoinResult) *tui.CallModel {
		log.Printf("Connecting to %s, Room: %s", *serverURL, res.RoomID)

		sigClient, err := signaling.NewClient(ctx, *serverURL, res.RoomID, res.Username)
		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		// Notice: we can't easily defer sigClient.Close() here because we are in a closure.
		// Instead, we let the context cancellation handle it when main exits.
		go func() {
			<-ctx.Done()
			sigClient.Close()
		}()

		mesh := rtc.NewMeshManager(sigClient)

		cam := capture.NewCamera(15)
		camChan, err := cam.Start(ctx)
		if err != nil {
			log.Fatalf("Failed to start camera: %v", err)
		}

		mic := capture.NewMicrophone()
		micTrack, err := mic.Start(ctx)
		if err != nil {
			log.Printf("Warning: failed to start microphone: %v", err)
		} else {
			mesh.SetLocalAudioTrack(micTrack)
			go func() {
				<-ctx.Done()
				mic.Close()
			}()
		}

		callModel := tui.NewCallModel(mesh, cam, mic)

		mesh.OnPeerJoin = func(peerID, username string) {
			send(tui.PeerJoinMsg{PeerID: peerID, Username: username})
		}

		mesh.OnPeerLeave = func(peerID string) {
			send(tui.PeerLeaveMsg{PeerID: peerID})
		}

		mesh.OnFrame = func(peerID string, frame string) {
			send(tui.PeerFrameMsg{PeerID: peerID, Frame: frame})
		}

		// Capture frames from camera
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case img, ok := <-camChan:
					if !ok {
						return
					}
					send(tui.LocalFrameMsg{RawImage: img})
				}
			}
		}()

		mesh.Start()
		go func() {
			<-ctx.Done()
			mesh.Stop()
		}()

		return callModel
	}

	app := tui.NewAppModel(skipForm, *roomID, *username, startCall)
	prog := tea.NewProgram(app, tea.WithAltScreen())
	p.Store(prog)

	if _, err := prog.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}

	log.Println("Shutting down...")
}
