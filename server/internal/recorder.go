package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const writeBufferSize = 64 * 1024 // 64KB buffer to batch writes and reduce syscalls

// RecorderManager handles H264 recording to MP4 file via ffmpeg
type RecorderManager struct {
	mu            sync.RWMutex
	recording     atomic.Bool
	ffmpegCmd     *exec.Cmd
	ffmpegStdin   io.WriteCloser
	writer        *bufio.Writer // Buffered writer to reduce syscalls
	filePath      string
	startTime     time.Time
	bytesWritten  int64
	framesWritten int64
	recordingDir  string
	naluChan      chan []byte
	done          chan struct{}
	wg            sync.WaitGroup

	// Cached keyframes for starting recordings
	lastSPS        []byte
	lastPPS        []byte
	waitingForIDR  bool // Flag to wait for keyframe before writing
}

// RecordingStatus represents the current recording state
type RecordingStatus struct {
	Available         bool   `json:"available"`
	Recording         bool   `json:"recording"`
	UnavailableReason string `json:"unavailableReason,omitempty"` // Reason why recording is unavailable
	FilePath          string `json:"filePath,omitempty"`
	StartTime         int64  `json:"startTime,omitempty"`
	DurationMs        int64  `json:"durationMs,omitempty"`
	BytesWritten      int64  `json:"bytesWritten,omitempty"`
	FramesWritten     int64  `json:"framesWritten,omitempty"`
}

// RecordingFile represents a recording file for listing
type RecordingFile struct {
	Filename   string `json:"filename"`
	SizeBytes  int64  `json:"sizeBytes"`
	CreatedAt  int64  `json:"createdAt"`
	DurationMs int64  `json:"durationMs"`
}

// RecordingMeta is metadata stored alongside each recording
type RecordingMeta struct {
	DurationMs int64 `json:"durationMs"`
	SizeBytes  int64 `json:"sizeBytes"`
}

// NewRecorderManager creates a new recorder instance
func NewRecorderManager(recordingDir string) *RecorderManager {
	return &RecorderManager{
		recordingDir: recordingDir,
		naluChan:     make(chan []byte, 500), // Buffer for burst tolerance
		done:         make(chan struct{}),
	}
}

// Start begins recording to a new MP4 file via ffmpeg
func (rm *RecorderManager) Start() (*RecordingStatus, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.recording.Load() {
		return nil, fmt.Errorf("recording already in progress")
	}

	// Verify we have SPS/PPS cached
	if rm.lastSPS == nil || rm.lastPPS == nil {
		return nil, fmt.Errorf("cannot start recording: SPS/PPS not yet available (wait for camera stream to initialize)")
	}

	// Generate filename with timestamp: recording_20260131_143052.mp4
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("recording_%s.mp4", timestamp)
	rm.filePath = filepath.Join(rm.recordingDir, filename)

	// Start ffmpeg to mux H.264 to MP4
	// -f h264: input format is raw H.264
	// -i pipe:0: read from stdin
	// -c copy: copy video stream without re-encoding
	// -movflags +faststart: optimize for web playback
	// -y: overwrite output file if exists
	rm.ffmpegCmd = exec.Command("ffmpeg",
		"-f", "h264",
		"-i", "pipe:0",
		"-c:v", "copy",
		"-movflags", "+faststart",
		"-y",
		rm.filePath,
	)

	// Suppress ffmpeg's verbose output (only show errors)
	rm.ffmpegCmd.Stderr = os.Stderr

	// Get stdin pipe to write H.264 data
	stdin, err := rm.ffmpegCmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create ffmpeg stdin pipe: %w", err)
	}
	rm.ffmpegStdin = stdin

	// Start ffmpeg process
	if err := rm.ffmpegCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	rm.writer = bufio.NewWriterSize(stdin, writeBufferSize)
	rm.startTime = time.Now()
	rm.bytesWritten = 0
	rm.framesWritten = 0

	// Write cached SPS/PPS first (required for decodable stream)
	n, _ := rm.writer.Write(rm.lastSPS)
	rm.bytesWritten += int64(n)
	n, _ = rm.writer.Write(rm.lastPPS)
	rm.bytesWritten += int64(n)

	// Set flag to wait for next IDR frame before writing any more data
	rm.waitingForIDR = true
	rm.recording.Store(true)

	log.Printf("Recording started (MP4), waiting for keyframe...")
	return rm.getStatusLocked(), nil
}

// Stop ends the current recording and finalizes the MP4 file
func (rm *RecorderManager) Stop() (*RecordingStatus, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.recording.Load() {
		return nil, fmt.Errorf("no recording in progress")
	}

	rm.recording.Store(false)

	status := rm.getStatusLocked()

	// Flush buffered data to ffmpeg
	if rm.writer != nil {
		rm.writer.Flush()
		rm.writer = nil
	}

	// Close stdin pipe to signal end of stream to ffmpeg
	if rm.ffmpegStdin != nil {
		rm.ffmpegStdin.Close()
		rm.ffmpegStdin = nil
	}

	// Wait for ffmpeg to finish muxing the MP4
	if rm.ffmpegCmd != nil {
		if err := rm.ffmpegCmd.Wait(); err != nil {
			log.Printf("ffmpeg exited with error: %v", err)
		}
		rm.ffmpegCmd = nil
	}

	// Write metadata file
	meta := RecordingMeta{
		DurationMs: status.DurationMs,
		SizeBytes:  status.BytesWritten,
	}
	metaPath := rm.filePath + ".meta"
	if metaData, err := json.Marshal(meta); err == nil {
		os.WriteFile(metaPath, metaData, 0644)
	}

	log.Printf("Recording stopped and MP4 finalized: %s", filepath.Base(rm.filePath))

	return status, nil
}

// GetStatus returns current recording status
func (rm *RecorderManager) GetStatus() *RecordingStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.getStatusLocked()
}

func (rm *RecorderManager) getStatusLocked() *RecordingStatus {
	status := &RecordingStatus{
		Available: true,
		Recording: rm.recording.Load(),
	}

	if status.Recording {
		status.FilePath = filepath.Base(rm.filePath)
		status.StartTime = rm.startTime.UnixMilli()
		status.DurationMs = time.Since(rm.startTime).Milliseconds()
		status.BytesWritten = rm.bytesWritten
		status.FramesWritten = rm.framesWritten
	}

	return status
}

// GetNALUChannel returns the channel for receiving NALUs
func (rm *RecorderManager) GetNALUChannel() chan<- []byte {
	return rm.naluChan
}

// ProcessNALUs starts the goroutine that writes NALUs to file
func (rm *RecorderManager) ProcessNALUs() {
	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()

		for {
			select {
			case nalu, ok := <-rm.naluChan:
				if !ok {
					return
				}
				rm.handleNALU(nalu)
			case <-rm.done:
				return
			}
		}
	}()
}

func (rm *RecorderManager) handleNALU(nalu []byte) {
	if len(nalu) < 5 {
		return // Invalid NALU
	}

	naluType := nalu[4] & 0x1F

	// Always cache SPS/PPS for starting future recordings
	if naluType == 7 { // SPS
		rm.mu.Lock()
		rm.lastSPS = make([]byte, len(nalu))
		copy(rm.lastSPS, nalu)
		rm.mu.Unlock()
	} else if naluType == 8 { // PPS
		rm.mu.Lock()
		rm.lastPPS = make([]byte, len(nalu))
		copy(rm.lastPPS, nalu)
		rm.mu.Unlock()
	}

	// If not recording, we're done (just cached SPS/PPS above if needed)
	if !rm.recording.Load() {
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.writer == nil {
		return
	}

	// If waiting for IDR frame, only start writing when we get one
	if rm.waitingForIDR {
		if naluType == 5 { // IDR frame
			rm.waitingForIDR = false
			log.Printf("Keyframe received, recording video stream...")
			// Write this IDR frame (fall through to write below)
		} else {
			// Skip non-IDR frames until we get a keyframe
			// This includes any SPS/PPS before the first IDR (we already wrote cached ones)
			return
		}
	}

	// Write the NALU to file
	n, err := rm.writer.Write(nalu)
	if err == nil {
		rm.bytesWritten += int64(n)
		rm.framesWritten++
	}
}

// ListRecordings returns all recording files in the recording directory
func (rm *RecorderManager) ListRecordings() ([]RecordingFile, error) {
	entries, err := os.ReadDir(rm.recordingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read recording directory: %w", err)
	}

	var recordings []RecordingFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Only include .mp4 files
		if filepath.Ext(name) != ".mp4" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		recording := RecordingFile{
			Filename:  name,
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UnixMilli(),
		}

		// Try to read duration from metadata file
		metaPath := filepath.Join(rm.recordingDir, name+".meta")
		if metaData, err := os.ReadFile(metaPath); err == nil {
			var meta RecordingMeta
			if json.Unmarshal(metaData, &meta) == nil {
				recording.DurationMs = meta.DurationMs
			}
		}

		recordings = append(recordings, recording)
	}

	return recordings, nil
}

// GetFilePath returns the full path to a recording file if it exists
func (rm *RecorderManager) GetFilePath(filename string) (string, error) {
	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)
	if filepath.Ext(filename) != ".mp4" {
		return "", fmt.Errorf("invalid file type")
	}

	fullPath := filepath.Join(rm.recordingDir, filename)

	// Check if file exists
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("file not found")
	}

	return fullPath, nil
}

// Shutdown gracefully shuts down the recorder
func (rm *RecorderManager) Shutdown() {
	close(rm.done)
	rm.wg.Wait()

	rm.mu.Lock()
	// If recording is in progress, stop it
	if rm.recording.Load() {
		rm.recording.Store(false)

		// Flush buffered data to ffmpeg
		if rm.writer != nil {
			rm.writer.Flush()
			rm.writer = nil
		}

		// Close stdin pipe to signal end of stream
		if rm.ffmpegStdin != nil {
			rm.ffmpegStdin.Close()
			rm.ffmpegStdin = nil
		}

		// Wait for ffmpeg to finish
		if rm.ffmpegCmd != nil {
			rm.ffmpegCmd.Wait()
			rm.ffmpegCmd = nil
		}
	}
	rm.mu.Unlock()

	close(rm.naluChan)
}
