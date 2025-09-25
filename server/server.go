package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	width  = 640
	height = 480
	fps    = 25
)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Println("New WebSocket connection from", r.RemoteAddr)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	peerClosed := make(chan struct{})

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("PeerConnection creation error:", err)
		ws.Close()
		return
	}

	track, err := newVideoTrack()
	if err != nil {
		log.Println("Track creation error:", err)
		pc.Close()
		ws.Close()
		return
	}

	if _, err := pc.AddTrack(track); err != nil {
		log.Println("AddTrack error:", err)
		pc.Close()
		ws.Close()
		return
	}

	// Send ICE candidates incrementally
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			ws.WriteJSON(map[string]interface{}{"candidate": c.ToJSON()})
		}
	})

	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Println("PeerConnection state:", s)
		if s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateFailed {
			close(peerClosed)
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State: %s", state.String())
	})

	// Handle WebSocket signaling
	go readWebSocketMessages(ws, pc, track)

	// Wait until PeerConnection closes
	<-peerClosed
	ws.Close()
}
