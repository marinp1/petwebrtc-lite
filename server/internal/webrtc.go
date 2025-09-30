// Package internal provides the core WebRTC logic for the webrtc-ipcam server.
//
// This file implements the main WebRTC signaling, media handling, and client management
// for streaming H264 video from a camera (such as a Raspberry Pi camera) to web clients
// using the Pion WebRTC library. It manages peer connections, video track creation,
// RTP packetization, and the broadcast of H264 NAL units to all connected clients.
//
// Key components:
//   - Client: Represents a single WebRTC client connection.
//   - ClientManager: Manages all active clients and handles NALU distribution.
//   - Camera process management: Starts and manages the camera streaming process.
//   - WebRTC signaling: Handles SDP offers/answers and ICE negotiation.
//   - RTP packetization: Converts H264 NALUs to RTP packets for WebRTC transport.
//
// Dependencies: Pion WebRTC, Pion RTP, rpicam-vid for camera streaming.
package internal

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
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
// (video frames) to them. It also provides methods to start the camera process
// and broadcast video data.
type ClientManager struct {
	Clients  map[*Client]struct{}
	NALUChan chan []byte
	Mu       sync.RWMutex
}

// NewClientManager creates and returns a new ClientManager instance.
func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients:  make(map[*Client]struct{}),
		NALUChan: make(chan []byte, 1000),
	}
}

// StartCamera launches the camera streaming process (e.g. rpicam-vid) using the provided shell command.
// It reads H264 NAL units from the process's stdout and sends them to the NALU channel for broadcasting.
func (cm *ClientManager) StartCamera(cameraCmd string) {
	// Kill any running or rpicam-vid processes before starting
	_ = exec.Command("pkill", "-9", "rpicam-vid").Run()

	cmd := exec.Command("sh", "-c", cameraCmd)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("stdout pipe error:", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal("failed to start camera:", err)
	}
	buf := make([]byte, 4096)
	var naluBuf []byte
	for {
		n, err := stdout.Read(buf)
		if err != nil {
			log.Println("camera stream ended:", err)
			break
		}
		naluBuf = append(naluBuf, buf[:n]...)
		for {
			start := FindNALUStart(naluBuf)
			if start == -1 {
				break
			}
			end := FindNALUStart(naluBuf[start+4:])
			if end == -1 {
				break
			}
			nalu := naluBuf[start : start+4+end]
			select {
			case cm.NALUChan <- nalu:
			default:
				// Drop if buffer full
			}
			naluBuf = naluBuf[start+4+end:]
		}
	}
}

// BroadcastNALUs reads H264 NAL units from the NALU channel and sends them as RTP packets
// to all connected clients with an active WebRTC connection.
func (cm *ClientManager) BroadcastNALUs() {
	for nalu := range cm.NALUChan {
		cm.Mu.RLock()
		for c := range cm.Clients {
			if c.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
				packets := c.Packetizer.Packetize(nalu, 90000/30)
				for _, pkt := range packets {
					_ = c.VideoTrack.WriteRTP(pkt)
				}
			}
		}
		cm.Mu.RUnlock()
	}
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
	cm.Mu.Lock()
	cm.Clients[client] = struct{}{}
	cm.Mu.Unlock()

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state: %v", state)
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			log.Println("Removing client and closing peer connection")
			cm.Mu.Lock()
			delete(cm.Clients, client)
			cm.Mu.Unlock()
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

// FindNALUStart searches for the start code (0x00000001) of an H264 NAL unit in the given buffer.
// Returns the index of the start code, or -1 if not found.
func FindNALUStart(buf []byte) int {
	for i := 0; i < len(buf)-3; i++ {
		if buf[i] == 0 && buf[i+1] == 0 && buf[i+2] == 0 && buf[i+3] == 1 {
			return i
		}
	}
	return -1
}
