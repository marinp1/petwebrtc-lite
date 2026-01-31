// Package internal provides the core WebRTC logic for the webrtc-ipcam server.
//
// This file implements camera process management and H264 NAL unit streaming.
// It handles launching the camera process (e.g., rpicam-vid), reading H264 NAL units
// from the camera output with optimizations for maximum local throughput.
//
// For local streaming, direct pipes are optimal. The real throughput gains come from:
//   - Large read buffers to minimize syscalls
//   - Efficient NALU parsing to avoid reprocessing data
//   - Non-blocking channel operations to prevent camera backpressure
//   - Proper buffer management to minimize allocations
package internal

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

// CameraManager manages the camera streaming process and H264 NAL unit distribution.
type CameraManager struct {
	NALUChan   chan []byte
	cmd        *exec.Cmd
	closer     io.Closer
	wg         sync.WaitGroup
	BufferSize int
	mu         sync.Mutex
	running    bool
}

// CameraConfig holds configuration for the camera manager
type CameraConfig struct {
	ChannelBuffer int // NALU channel buffer size (default: 2000)
	ReadBuffer    int // Read buffer size in bytes (default: 256KB)
}

// NewCameraManager creates and returns a new CameraManager instance with the given config.
func NewCameraManager(config CameraConfig) *CameraManager {
	channelBuffer := config.ChannelBuffer
	if channelBuffer == 0 {
		channelBuffer = 2000 // Larger buffer to handle bursts
	}

	readBuffer := config.ReadBuffer
	if readBuffer == 0 {
		readBuffer = 256 * 1024 // 256KB - sweet spot for video streaming
	}

	return &CameraManager{
		NALUChan:   make(chan []byte, channelBuffer),
		BufferSize: readBuffer,
	}
}

// StartCamera launches the camera streaming process using the provided shell command.
// It reads H264 NAL units from stdout and sends them to the NALU channel for broadcasting.
// Returns an error if camera is already running or fails to start.
func (cm *CameraManager) StartCamera(cameraCmd string) error {
	cm.mu.Lock()
	if cm.running {
		cm.mu.Unlock()
		return fmt.Errorf("camera is already running")
	}
	cm.running = true
	cm.mu.Unlock()

	// Kill any running rpicam-vid processes before starting
	_ = exec.Command("pkill", "-9", "rpicam-vid").Run()

	cm.cmd = exec.Command("sh", "-c", cameraCmd)
	cm.cmd.Stderr = os.Stderr

	stdout, err := cm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}
	cm.closer = stdout

	if err := cm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start camera: %w", err)
	}

	log.Println("Camera process started, streaming H264...")

	// Start reading in goroutine
	cm.wg.Add(1)
	go cm.readStream(stdout)

	return nil
}

// readStream reads H264 data from stdout and extracts NAL units with optimized buffering
func (cm *CameraManager) readStream(reader io.Reader) {
	defer cm.wg.Done()

	// Use buffered reader for efficient reading
	bufReader := bufio.NewReaderSize(reader, cm.BufferSize)

	// Pre-allocate NALU buffer with reasonable capacity
	naluBuf := make([]byte, 0, cm.BufferSize*2)
	readBuf := make([]byte, cm.BufferSize)

	// Statistics for monitoring
	var totalNALUs, droppedNALUs uint64

	for {
		n, err := bufReader.Read(readBuf)
		if n > 0 {
			naluBuf = append(naluBuf, readBuf[:n]...)

			// Extract all complete NAL units from buffer
			cm.extractNALUs(&naluBuf, &totalNALUs, &droppedNALUs)
		}

		if err != nil {
			if err != io.EOF {
				log.Printf("Stream read error: %v", err)
			} else {
				log.Println("Camera stream ended normally")
			}
			break
		}
	}

	// Log statistics
	if droppedNALUs > 0 {
		dropRate := float64(droppedNALUs) / float64(totalNALUs) * 100
		log.Printf("Camera stats - Total NALUs: %d, Dropped: %d (%.2f%%)",
			totalNALUs, droppedNALUs, dropRate)
	} else {
		log.Printf("Camera stats - Total NALUs: %d, No drops", totalNALUs)
	}
}

// extractNALUs efficiently extracts complete NAL units from the buffer
func (cm *CameraManager) extractNALUs(naluBuf *[]byte, totalNALUs, droppedNALUs *uint64) {
	buf := *naluBuf

	for len(buf) > 4 {
		// Find first NAL unit start
		start := findNALUStart(buf)
		if start == -1 {
			// No start code found, keep last 3 bytes in case split across reads
			if len(buf) > 3 {
				copy(buf[:3], buf[len(buf)-3:])
				*naluBuf = buf[:3]
			}
			return
		}

		// Discard data before start code
		if start > 0 {
			buf = buf[start:]
		}

		// Find next NAL unit start
		nextStart := findNALUStart(buf[4:])
		if nextStart == -1 {
			// Incomplete NAL unit, keep for next iteration
			*naluBuf = buf
			return
		}

		// Extract complete NAL unit
		naluLen := 4 + nextStart
		nalu := make([]byte, naluLen)
		copy(nalu, buf[:naluLen])

		*totalNALUs++

		// Non-blocking send - prioritize keeping camera flowing
		select {
		case cm.NALUChan <- nalu:
			// Sent successfully
		default:
			// Channel full - drop oldest frame
			*droppedNALUs++
			select {
			case <-cm.NALUChan:
				cm.NALUChan <- nalu
			default:
				// Even oldest couldn't be dropped, skip this frame
			}
		}

		// Move to next NAL unit
		buf = buf[naluLen:]
	}

	// Keep remaining bytes for next read
	*naluBuf = buf
}

// findNALUStart performs optimized search for H264 NAL unit start code
// Uses early rejection to minimize comparisons
func findNALUStart(buf []byte) int {
	if len(buf) < 4 {
		return -1
	}

	// Optimized loop with early rejection
	for i := 0; i <= len(buf)-4; i++ {
		// Most bytes aren't 0, so check this first
		if buf[i] != 0 {
			continue
		}

		// Second byte also likely not 0
		if buf[i+1] != 0 {
			continue
		}

		// Check for 4-byte start code: 0x00 00 00 01
		if buf[i+2] == 0 && buf[i+3] == 1 {
			return i
		}

		// Check for 3-byte start code: 0x00 00 01
		// (some encoders use this)
		if buf[i+2] == 1 {
			return i
		}
	}

	return -1
}

// GetNALUChannel returns the channel for receiving H264 NAL units.
func (cm *CameraManager) GetNALUChannel() <-chan []byte {
	return cm.NALUChan
}

// Stop gracefully stops the camera process and waits for cleanup
func (cm *CameraManager) Stop() error {
	cm.mu.Lock()
	if !cm.running {
		cm.mu.Unlock()
		return nil
	}
	cm.mu.Unlock()

	if cm.cmd == nil || cm.cmd.Process == nil {
		return nil
	}

	log.Println("Stopping camera process...")

	// Try graceful shutdown first
	if err := cm.cmd.Process.Signal(os.Interrupt); err != nil {
		// If that fails, force kill
		_ = cm.cmd.Process.Kill()
	}

	// Wait for read goroutine to finish
	cm.wg.Wait()

	// Close channel
	close(cm.NALUChan)

	cm.mu.Lock()
	cm.running = false
	cm.mu.Unlock()

	log.Println("Camera stopped")
	return nil
}
