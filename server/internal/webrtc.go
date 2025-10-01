// Package internal provides the core WebRTC logic for the webrtc-ipcam server.
//
// This file implements WebRTC signaling, media handling, and client management
// for streaming H264 video to web clients using the Pion WebRTC library.
// It manages peer connections, video track creation, RTP packetization,
// and the broadcast of H264 NAL units to all connected clients.
//
// Key components:
//   - Client: Represents a single WebRTC client connection.
//   - ClientManager: Manages all active clients and handles NALU distribution.
//   - WebRTC signaling: Handles SDP offers/answers and ICE negotiation.
//   - RTP packetization: Converts H264 NALUs to RTP packets for WebRTC transport.
//
// Dependencies: Pion WebRTC, Pion RTP
package internal

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"webrtc-ipcam/config"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

// Client represents a single WebRTC client connection, including its peer connection,
// video track, and RTP packetizer for sending H264 video.
type Client struct {
	PeerConn   *webrtc.PeerConnection
	VideoTrack *webrtc.TrackLocalStaticRTP
	Packetizer rtp.Packetizer
}

// ClientManager manages all active WebRTC clients and distributes H264 NAL units
// (video frames) to them.
type ClientManager struct {
	Clients map[*Client]struct{}
	Mu      sync.RWMutex
}

// NewClientManager creates and returns a new ClientManager instance.
func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients: make(map[*Client]struct{}),
	}
}

// BroadcastNALUs reads H264 NAL units from the provided NALU channel and sends them as RTP packets
// to all connected clients with an active WebRTC connection.
func (cm *ClientManager) BroadcastNALUs(naluChan <-chan []byte) {
	for nalu := range naluChan {
		// Copy client list under lock
		cm.Mu.RLock()
		clients := make([]*Client, 0, len(cm.Clients))
		for c := range cm.Clients {
			clients = append(clients, c)
		}
		cm.Mu.RUnlock()
		// Send RTP packets outside lock
		for _, c := range clients {
			if c.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
				packets := c.Packetizer.Packetize(nalu, 90000/30)
				for _, pkt := range packets {
					_ = c.VideoTrack.WriteRTP(pkt)
				}
			}
		}
	}
}

// AddClient adds a new client to the manager.
func (cm *ClientManager) AddClient(client *Client) {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()
	cm.Clients[client] = struct{}{}
}

// RemoveClient removes a client from the manager.
func (cm *ClientManager) RemoveClient(client *Client) {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()
	delete(cm.Clients, client)
}

// SetupMediaEngine configures and returns a Pion MediaEngine with H264 video codec support.
func SetupMediaEngine() *webrtc.MediaEngine {
	m := &webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "profile-level-id=42e01f;level-asymmetry-allowed=1;packetization-mode=1",
			RTCPFeedback: nil,
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo)
	_ = m.RegisterDefaultCodecs()
	return m
}

// HandleOffer handles incoming WebRTC SDP offers from clients, sets up a new peer connection,
// creates a video track, and responds with an SDP answer. It also manages client lifecycle events.
func HandleOffer(w http.ResponseWriter, r *http.Request, api *webrtc.API, cm *ClientManager, conf *config.ServerConfig) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "invalid offer", http.StatusBadRequest)
		return
	}
	log.Printf("Received offer SDP:\n%s", offer.SDP)

	peerConn, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	})
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
	_, err = peerConn.AddTrack(videoTrack)
	if err != nil {
		http.Error(w, "failed to add track", http.StatusInternalServerError)
		return
	}

	ssrc := rand.Uint32()
	packetizer := rtp.NewPacketizer(
		1200, 96, ssrc, &codecs.H264Payloader{},
		rtp.NewRandomSequencer(), 90000,
	)

	client := &Client{peerConn, videoTrack, packetizer}
	cm.AddClient(client)

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state: %v", state)
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			log.Println("Removing client and closing peer connection")
			cm.RemoveClient(client)
			peerConn.Close()
		}
	})

	if err := peerConn.SetRemoteDescription(offer); err != nil {
		http.Error(w, "failed to set remote description", http.StatusInternalServerError)
		log.Printf("SetRemoteDescription error: %v", err)
		return
	}

	answer, err := peerConn.CreateAnswer(nil)
	if err != nil {
		http.Error(w, "failed to create answer", http.StatusInternalServerError)
		return
	}
	if err := peerConn.SetLocalDescription(answer); err != nil {
		http.Error(w, "failed to set local description", http.StatusInternalServerError)
		return
	}
	log.Printf("Sending answer SDP:\n%s", answer.SDP)

	gatherDone := make(chan struct{})
	peerConn.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		if state == webrtc.ICEGatheringStateComplete {
			close(gatherDone)
		}
	})
	if peerConn.ICEGatheringState() == webrtc.ICEGatheringStateComplete {
		close(gatherDone)
	}
	select {
	case <-gatherDone:
	case <-time.After(2 * time.Second):
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(peerConn.LocalDescription())
}
