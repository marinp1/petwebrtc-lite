package internal

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"webrtc-ipcam/config"

	"github.com/pion/webrtc/v4"
)

func SetupMediaEngine() *webrtc.MediaEngine {
	m := &webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "profile-level-id=42e01f;level-asymmetry-allowed=1;packetization-mode=1",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo)
	_ = m.RegisterDefaultCodecs()
	return m
}

func HandleOffer(w http.ResponseWriter, r *http.Request, api *webrtc.API, cm *ClientManager, conf *config.ServerConfig) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "invalid offer", http.StatusBadRequest)
		return
	}
	log.Printf("Received offer SDP:\n%s", offer.SDP)

	peerConn, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, "failed to create peer connection", http.StatusInternalServerError)
		return
	}

	// Ensure cleanup on error paths
	var setupComplete bool
	defer func() {
		if !setupComplete {
			peerConn.Close()
		}
	}()

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video", "rpi-camera",
	)
	if err != nil {
		http.Error(w, "failed to create track", http.StatusInternalServerError)
		return
	}

	if _, err := peerConn.AddTrack(videoTrack); err != nil {
		log.Printf("Failed to add track: %v", err)
		http.Error(w, "failed to add track", http.StatusInternalServerError)
		return
	}

	// pass framerate so client timestamps increment consistently
	client := NewClient(peerConn, videoTrack, nil, conf.Framerate)

	// Handle incoming data channel from client
	peerConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("Data channel received from client: %s", dc.Label())

		// Update this specific client's data channel safely
		client.SetDataChannel(dc)

		dc.OnOpen(func() {
			log.Println("Data channel opened")
		})

		dc.OnClose(func() {
			log.Println("Data channel closed")
		})

		dc.OnError(func(err error) {
			log.Printf("Data channel error: %v", err)
		})
	})

	cm.AddClient(client)

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state: %v", state)
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			cm.RemoveClient(client)
			peerConn.Close()
		}
	})

	peerConn.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		log.Printf("New ICE candidate: %s", c.String())
	})

	peerConn.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE connection state: %s", state.String())
	})

	if err := peerConn.SetRemoteDescription(offer); err != nil {
		http.Error(w, "failed to set remote description", http.StatusInternalServerError)
		return
	}

	answer, err := peerConn.CreateAnswer(nil)
	if err != nil {
		http.Error(w, "failed to create answer", http.StatusInternalServerError)
		return
	}

	if err := peerConn.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		http.Error(w, "failed to set local description", http.StatusInternalServerError)
		return
	}

	// Wait for ICE candidates (grow timeout to be more tolerant on slow networks)
	done := make(chan struct{})
	peerConn.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		if state == webrtc.ICEGatheringStateComplete {
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second): // increased timeout
	}

	// Mark setup as complete to prevent cleanup
	setupComplete = true

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(peerConn.LocalDescription()); err != nil {
		log.Printf("Failed to encode answer: %v", err)
	}
}
