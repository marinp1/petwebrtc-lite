package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	width  = 640
	height = 480
	fps    = 25
)

//go:embed public/*
var content embed.FS

func main() {
	// Kill previous camera/ffmpeg processes
	exec.Command("pkill", "-f", "rpicam-vid").Run()
	exec.Command("pkill", "-f", "ffmpeg").Run()

	publicFS, err := fs.Sub(content, "public")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(publicFS)))
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("Server running at http://0.0.0.0:8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Println("New WebSocket connection from", r.RemoteAddr)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	// Channels to track PeerConnection and signaling
	peerClosed := make(chan struct{})

	// Create PeerConnection with minimal STUN configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("PeerConnection creation error:", err)
		ws.Close()
		return
	}

	// Create H264 track
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video",
		"pion",
	)
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

	// Send ICE candidates to client
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			log.Printf("Sending ICE candidate: %s", c.ToJSON().Candidate)
			ws.WriteJSON(map[string]interface{}{"candidate": c.ToJSON()})
		}
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Println("PeerConnection state:", s)
		if s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateFailed {
			exec.Command("pkill", "-f", "rpicam-vid").Run()
			exec.Command("pkill", "-f", "ffmpeg").Run()
			close(peerClosed)
		}
	})

	// Handle ICE connection state changes for debugging
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State: %s", connectionState.String())
	})

	// Read WebSocket messages (signaling)
	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				pc.Close() // trigger PeerConnection close
				return
			}
			if err := handleSignaling(pc, ws, string(msg), track); err != nil {
				log.Println("Signaling error:", err)
			}
		}
	}()

	// Wait until PeerConnection closes before closing WebSocket
	<-peerClosed
	ws.Close()
}

// handleSignaling processes SDP offers and ICE candidates
func handleSignaling(pc *webrtc.PeerConnection, ws *websocket.Conn, msg string, track *webrtc.TrackLocalStaticSample) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &data); err != nil {
		return fmt.Errorf("JSON unmarshal error: %w", err)
	}

	// SDP offer
	if sdpMap, ok := data["sdp"].(map[string]interface{}); ok {
		sdpType, ok1 := sdpMap["type"].(string)
		sdpStr, ok2 := sdpMap["sdp"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("invalid SDP format")
		}

		log.Printf("Received SDP %s", sdpType)
		log.Printf("SDP content: %s", sdpStr)

		desc := webrtc.SessionDescription{
			Type: webrtc.NewSDPType(sdpType),
			SDP:  sdpStr,
		}

		// Set remote description (client offer)
		if err := pc.SetRemoteDescription(desc); err != nil {
			return fmt.Errorf("SetRemoteDescription error: %w", err)
		}

		// Create answer
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			return fmt.Errorf("CreateAnswer error: %w", err)
		}
		if err := pc.SetLocalDescription(answer); err != nil {
			return fmt.Errorf("SetLocalDescription error: %w", err)
		}

		// Send SDP answer to client
		log.Println("Sending SDP answer to client")
		if err := ws.WriteJSON(map[string]interface{}{"sdp": pc.LocalDescription()}); err != nil {
			return fmt.Errorf("Failed to send SDP answer: %w", err)
		}

		// Start camera streaming now
		go startCameraStream(track)

		return nil
	}

	// ICE candidate
	if candMap, ok := data["candidate"].(map[string]interface{}); ok {
		cStr, ok := candMap["candidate"].(string)
		if !ok {
			return fmt.Errorf("invalid ICE candidate format")
		}
		log.Printf("Received ICE candidate: %s", cStr)
		if err := pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: cStr}); err != nil {
			return fmt.Errorf("AddICECandidate error: %w", err)
		}
		return nil
	}

	return fmt.Errorf("unknown signaling message")
}

// startCameraStream launches rpicam-vid + ffmpeg and pushes to WebRTC track
func startCameraStream(track *webrtc.TrackLocalStaticSample) {
	log.Println("Starting camera stream...")

	camera := exec.Command("rpicam-vid",
		"-t", "0",
		"--width", fmt.Sprint(width),
		"--height", fmt.Sprint(height),
		"--framerate", fmt.Sprint(fps),
		"--inline",
		"-o", "-",
	)
	ffmpeg := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-c:v", "copy",
		"-f", "h264",
		"pipe:1",
	)

	cameraStdout, _ := camera.StdoutPipe()
	ffmpeg.Stdin = cameraStdout
	ffmpegStdout, _ := ffmpeg.StdoutPipe()
	ffmpeg.Stderr = os.Stderr

	if err := camera.Start(); err != nil {
		log.Printf("Failed to start camera: %v", err)
		return
	}
	if err := ffmpeg.Start(); err != nil {
		log.Printf("Failed to start ffmpeg: %v", err)
		camera.Process.Kill()
		return
	}

	buf := make([]byte, 4096)
	nalBuf := []byte{}

	for {
		n, err := ffmpegStdout.Read(buf)
		if err != nil {
			log.Println("FFmpeg read error:", err)
			break
		}
		nalBuf = append(nalBuf, buf[:n]...)

		for {
			start := bytes.Index(nalBuf, []byte{0, 0, 0, 1})
			if start == -1 || len(nalBuf) <= start+4 {
				break
			}
			nextStart := bytes.Index(nalBuf[start+4:], []byte{0, 0, 0, 1})
			if nextStart == -1 {
				break
			}
			nalUnit := nalBuf[start : start+4+nextStart]
			nalBuf = nalBuf[start+4+nextStart:]

			sample := media.Sample{
				Data:     nalUnit,
				Duration: time.Second / fps,
			}
			if err := track.WriteSample(sample); err != nil {
				log.Println("WriteSample error:", err)
			}
		}
	}

	// Cleanup
	camera.Process.Kill()
	ffmpeg.Process.Kill()
	log.Println("Camera stream stopped")
}
