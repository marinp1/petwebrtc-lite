// Package internal provides the core WebRTC logic for the webrtc-ipcam server.
//
// This file implements camera process management and H264 NAL unit streaming.
// It handles launching the camera process (e.g., rpicam-vid), reading H264 NAL units
// from the camera output, and distributing them via a channel for broadcasting.
package internal

import (
	"log"
	"os"
	"os/exec"
)

// CameraManager manages the camera streaming process and H264 NAL unit distribution.
type CameraManager struct {
	NALUChan chan []byte
}

// NewCameraManager creates and returns a new CameraManager instance.
func NewCameraManager() *CameraManager {
	return &CameraManager{
		NALUChan: make(chan []byte, 500), // Increased buffer for higher throughput
	}
}

// StartCamera launches the camera streaming process (e.g. rpicam-vid) using the provided shell command.
// It reads H264 NAL units from the process's stdout and sends them to the NALU channel for broadcasting.
func (cm *CameraManager) StartCamera(cameraCmd string) {
	// Kill any running rpicam-vid processes before starting
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
				// Sent successfully
			default:
				// Buffer full: drop oldest and insert new
				<-cm.NALUChan
				cm.NALUChan <- nalu
			}
			naluBuf = naluBuf[start+4+end:]
		}
	}
}

// GetNALUChannel returns the channel for receiving H264 NAL units.
func (cm *CameraManager) GetNALUChannel() <-chan []byte {
	return cm.NALUChan
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
