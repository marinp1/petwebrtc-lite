package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

func newVideoTrack() (*webrtc.TrackLocalStaticSample, error) {
	return webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video",
		"pion",
	)
}

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

	camera.Process.Kill()
	ffmpeg.Process.Kill()
	log.Println("Camera stream stopped")
}
