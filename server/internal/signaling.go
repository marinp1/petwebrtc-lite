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

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video", "rpi-camera",
	)
	if err != nil {
		http.Error(w, "failed to create track", http.StatusInternalServerError)
		return
	}
	_, _ = peerConn.AddTrack(videoTrack)

	// Handle incoming data channel from client
	peerConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("Data channel received from client: %s", dc.Label())

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

	client := NewClient(peerConn, videoTrack, nil) // Will be updated when data channel opens
	cm.AddClient(client)

	// Update client's data channel reference when it opens
	peerConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		client.DataChannel = dc
	})

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state: %v", state)
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			cm.RemoveClient(client)
			peerConn.Close()
		}
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
	_ = peerConn.SetLocalDescription(answer)

	// Wait for ICE candidates
	done := make(chan struct{})
	peerConn.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		if state == webrtc.ICEGatheringStateComplete {
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(peerConn.LocalDescription())
}
