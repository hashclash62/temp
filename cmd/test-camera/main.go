package main

import (
	"context"
	"fmt"
	"time"

	"github.com/meow/termcall/internal/ascii"
	"github.com/meow/termcall/internal/capture"
)

func main() {
	cam := capture.NewCamera(15)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Starting camera...")
	frameChan, err := cam.Start(ctx)
	if err != nil {
		fmt.Printf("Error starting camera: %v\n", err)
		return
	}

	renderer := ascii.NewDefaultRenderer(ascii.Config{})

	framesReceived := 0
	for {
		select {
		case frame, ok := <-frameChan:
			if !ok {
				fmt.Println("Frame channel closed.")
				return
			}
			framesReceived++
			if framesReceived == 1 {
				fmt.Println("--- FIRST FRAME PREVIEW ---")
				asciiFrame := renderer.Convert(frame, 80, 24)
				fmt.Println(asciiFrame)
				fmt.Println("---------------------------")
			}
		case <-ctx.Done():
			fmt.Printf("Test finished. Received %d frames.\n", framesReceived)
			return
		}
	}
}
