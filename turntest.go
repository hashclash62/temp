package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/pion/turn/v4"
)

func main() {
	turnIP := "13.127.137.230"
	turnPort := 8443
	turnUser := "testuser"
	turnPass := "termcall"
	realm := "termcall.local"

	fmt.Println("Testing TURN over UDP...")
	connUDP, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		log.Fatal(err)
	}
	clientUDP, err := turn.NewClient(&turn.ClientConfig{
		STUNServerAddr: fmt.Sprintf("%s:%d", turnIP, turnPort),
		TURNServerAddr: fmt.Sprintf("%s:%d", turnIP, turnPort),
		Conn:           connUDP,
		Username:       turnUser,
		Password:       turnPass,
		Realm:          realm,
	})
	if err != nil {
		log.Printf("Failed to create UDP client: %v", err)
	} else {
		err = clientUDP.Listen()
		if err != nil {
			log.Printf("UDP Listen failed: %v", err)
		} else {
			relayConn, err := clientUDP.Allocate()
			if err != nil {
				log.Printf("UDP Allocate failed: %v", err)
			} else {
				log.Printf("UDP Allocate SUCCESS: relay address %s", relayConn.LocalAddr())
			}
		}
		clientUDP.Close()
	}

	fmt.Println("\nTesting TURN over TCP...")
	connTCP, err := net.DialTimeout("tcp4", fmt.Sprintf("%s:%d", turnIP, turnPort), 5*time.Second)
	if err != nil {
		log.Printf("Failed to dial TCP: %v", err)
	} else {
		clientTCP, err := turn.NewClient(&turn.ClientConfig{
			STUNServerAddr: fmt.Sprintf("%s:%d", turnIP, turnPort),
			TURNServerAddr: fmt.Sprintf("%s:%d", turnIP, turnPort),
			Conn:           turn.NewSTUNConn(connTCP),
			Username:       turnUser,
			Password:       turnPass,
			Realm:          realm,
		})
		if err != nil {
			log.Printf("Failed to create TCP client: %v", err)
		} else {
			err = clientTCP.Listen()
			if err != nil {
				log.Printf("TCP Listen failed: %v", err)
			} else {
				relayConn, err := clientTCP.Allocate()
				if err != nil {
					log.Printf("TCP Allocate failed: %v", err)
				} else {
					log.Printf("TCP Allocate SUCCESS: relay address %s", relayConn.LocalAddr())
				}
			}
			clientTCP.Close()
		}
	}
}
