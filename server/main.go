package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
)

//go:embed public/*
var publicFS embed.FS

func main() {
	m := &webrtc.MediaEngine{}
	// Explicitly register H264 codec with payload type 96
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
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Fatal("failed to register codecs:", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(subFS)))

	http.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		var offer webrtc.SessionDescription
		if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
			http.Error(w, "invalid offer", http.StatusBadRequest)
			return
		}
		log.Printf("Received offer SDP:\n%s", offer.SDP)

		peerConn, err := api.NewPeerConnection(webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
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

		// Wait for ICE gathering to complete before sending answer
		gatherDone := make(chan struct{})
		peerConn.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
			if state == webrtc.ICEGathererStateComplete {
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

		// Start streaming from the Raspberry Pi camera
		go func() {
			// Output raw H.264 to stdout
			cmd := exec.Command("sh", "-c",
				"rpicam-vid -t 0 --width 1280 --height 720 --framerate 30 --inline --rotation 180 --codec h264 --nopreview -o -",
			)
			cmd.Stderr = os.Stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				log.Println("stdout pipe error:", err)
				return
			}
			if err := cmd.Start(); err != nil {
				log.Println("failed to start rpicam-vid:", err)
				return
			}

			ssrc := rand.Uint32() // Generate a random SSRC

			// Use pion RTP packetizer for H264
			packetizer := rtp.NewPacketizer(
				1200, // MTU
				96,   // PayloadType
				ssrc, // Use generated SSRC
				&codecs.H264Payloader{},
				rtp.NewRandomSequencer(),
				90000,
			)

			buf := make([]byte, 4096)
			var naluBuf []byte
			for {
				n, err := stdout.Read(buf)
				if err != nil {
					log.Println("stream ended:", err)
					return
				}
				if peerConn.ConnectionState() != webrtc.PeerConnectionStateConnected {
					time.Sleep(time.Millisecond * 50)
					continue
				}
				naluBuf = append(naluBuf, buf[:n]...)
				for {
					start := findNALUStart(naluBuf)
					if start == -1 {
						break
					}
					end := findNALUStart(naluBuf[start+4:])
					if end == -1 {
						break
					}
					nalu := naluBuf[start : start+4+end]
					packets := packetizer.Packetize(nalu, 90000/30)
					for _, pkt := range packets {
						if err := videoTrack.WriteRTP(pkt); err != nil {
							log.Println("write RTP error:", err)
						}
					}
					naluBuf = naluBuf[start+4+end:]
				}
			}
		}()
	})

	log.Println("WebRTC server running on :8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}

// Helper to find NALU start code (0x00000001)
func findNALUStart(buf []byte) int {
	for i := 0; i < len(buf)-3; i++ {
		if buf[i] == 0 && buf[i+1] == 0 && buf[i+2] == 0 && buf[i+3] == 1 {
			return i
		}
	}
	return -1
}
