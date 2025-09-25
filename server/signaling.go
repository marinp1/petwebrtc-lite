package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

func readWebSocketMessages(ws *websocket.Conn, pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticSample) {
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			pc.Close()
			return
		}
		log.Println("WebSocket message received:", string(msg))
		if err := handleSignaling(pc, ws, string(msg), track); err != nil {
			log.Println("Signaling error:", err)
		}
	}
}

var remoteCandidates []webrtc.ICECandidateInit
var remoteDescSet bool

func handleSignaling(pc *webrtc.PeerConnection, ws *websocket.Conn, msg string, track *webrtc.TrackLocalStaticSample) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &data); err != nil {
		return fmt.Errorf("JSON unmarshal error: %w", err)
	}

	// Handle SDP
	if sdpMap, ok := data["sdp"].(map[string]interface{}); ok {
		sdpType, _ := sdpMap["type"].(string)
		sdpStr, _ := sdpMap["sdp"].(string)

		desc := webrtc.SessionDescription{
			Type: webrtc.NewSDPType(sdpType),
			SDP:  sdpStr,
		}

		if err := pc.SetRemoteDescription(desc); err != nil {
			return fmt.Errorf("SetRemoteDescription error: %w", err)
		}
		remoteDescSet = true

		// Add queued ICE candidates
		for _, c := range remoteCandidates {
			if err := pc.AddICECandidate(c); err != nil {
				log.Println("Failed to add queued candidate:", err)
			}
		}
		remoteCandidates = nil
		return nil
	}

	// Handle ICE candidate
	if candMap, ok := data["candidate"].(map[string]interface{}); ok {
		cStr, _ := candMap["candidate"].(string)
		sdpMid, _ := candMap["sdpMid"].(string)
		sdpMLineIndex := uint16(candMap["sdpMLineIndex"].(float64))

		cand := webrtc.ICECandidateInit{
			Candidate:     cStr,
			SDPMid:        &sdpMid,
			SDPMLineIndex: &sdpMLineIndex,
		}

		if remoteDescSet {
			if err := pc.AddICECandidate(cand); err != nil {
				return fmt.Errorf("AddICECandidate error: %w", err)
			}
		} else {
			// Queue candidate until remote description is set
			remoteCandidates = append(remoteCandidates, cand)
		}
		return nil
	}

	return fmt.Errorf("unknown signaling message")
}
