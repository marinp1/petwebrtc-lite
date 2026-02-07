package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const writeBufferSize = 64 * 1024 // 64KB buffer to batch writes and reduce syscalls

// RecorderManager handles H264 recording (writes .h264, converts to MP4 afterward)
type RecorderManager struct {
	mu             sync.RWMutex
	recording      atomic.Bool
	finalizing     atomic.Bool // Set during MP4 conversion
	file           *os.File
	writer         *bufio.Writer // Buffered writer to reduce syscalls
	tempH264Path   string        // Path to raw .h264 file during recording
	finalH264Path  string        // Path after rename when recording complete
	filePath       string        // Path to final .mp4 file
	skipConversion bool

	startTime     time.Time
	bytesWritten  int64
	framesWritten int64
	recordingDir  string
	naluChan      chan []byte
	done          chan struct{}
	wg            sync.WaitGroup

	// Cached keyframes for starting recordings
	lastSPS       []byte
	lastPPS       []byte
	waitingForIDR bool // Flag to wait for keyframe before writing

	maxDuration time.Duration   // Maximum recording duration
	stopTimer   *time.Timer     // Timer to auto-stop recording at max duration
}

// RecordingStatus represents the current recording state
type RecordingStatus struct {
	Available         bool   `json:"available"`
	Recording         bool   `json:"recording"`
	Finalizing        bool   `json:"finalizing"`                  // True during MP4 conversion
	UnavailableReason string `json:"unavailableReason,omitempty"` // Reason why recording is unavailable
	FilePath          string `json:"filePath,omitempty"`
	StartTime         int64  `json:"startTime,omitempty"`
	DurationMs        int64  `json:"durationMs,omitempty"`
	MaxDurationMs     int64  `json:"maxDurationMs"`               // Max recording duration in ms
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
func NewRecorderManager(recordingDir string, skipConversion bool, maxMinutes int) *RecorderManager {
	return &RecorderManager{
		recordingDir:   recordingDir,
		skipConversion: skipConversion,
		maxDuration:    time.Duration(maxMinutes) * time.Minute,
		naluChan:       make(chan []byte, 500), // Buffer for burst tolerance
		done:           make(chan struct{}),
	}
}

// Start begins recording to a new .h264 file (converts to MP4 on stop)
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

	// Generate filenames with timestamp
	timestamp := time.Now().Format("20060102_150405")
	h264FinalFilename := fmt.Sprintf("recording_%s.h264", timestamp)
	h264TemporaryFilename := fmt.Sprintf("%s.tmp", h264FinalFilename)
	rm.finalH264Path = filepath.Join(rm.recordingDir, h264FinalFilename)
	rm.tempH264Path = filepath.Join(rm.recordingDir, h264TemporaryFilename)
	rm.filePath = filepath.Join(rm.recordingDir, fmt.Sprintf("recording_%s.mp4", timestamp))

	// Create .h264 file for raw recording
	file, err := os.Create(rm.tempH264Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	rm.file = file
	rm.writer = bufio.NewWriterSize(file, writeBufferSize)
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

	// Start auto-stop timer
	rm.stopTimer = time.AfterFunc(rm.maxDuration, func() {
		log.Printf("Recording reached max duration (%v), auto-stopping...", rm.maxDuration)
		if _, err := rm.Stop(); err != nil {
			log.Printf("Failed to auto-stop recording: %v", err)
		}
	})

	log.Printf("Recording started (.h264), max duration %v, waiting for keyframe...", rm.maxDuration)
	return rm.getStatusLocked(), nil
}

// Stop ends the current recording, converts .h264 to MP4, and cleans up
// Mutex is held during entire operation including MP4 conversion to prevent parallel recordings
func (rm *RecorderManager) Stop() (*RecordingStatus, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.recording.Load() {
		return nil, fmt.Errorf("no recording in progress")
	}

	if rm.finalizing.Load() {
		return nil, fmt.Errorf("recording is already finalizing")
	}

	rm.recording.Store(false)

	// Cancel auto-stop timer if running
	if rm.stopTimer != nil {
		rm.stopTimer.Stop()
		rm.stopTimer = nil
	}

	status := rm.getStatusLocked()

	// Flush and close .h264 file
	if rm.writer != nil {
		rm.writer.Flush()
		rm.writer = nil
	}
	if rm.file != nil {
		rm.file.Sync()
		rm.file.Close()
		rm.file = nil
	}

	// Set finalizing state (mutex still held, prevents new recordings)
	rm.finalizing.Store(true)
	defer rm.finalizing.Store(false)

	// Rename the temp files
	if err := os.Rename(rm.tempH264Path, rm.finalH264Path); err != nil {
		return nil, fmt.Errorf("failed to rename file: %w (file %s)", err, rm.tempH264Path)
	}

	log.Printf("Recording stopped: %s (%d bytes, %dms)", filepath.Base(rm.finalH264Path), status.BytesWritten, status.DurationMs)

	// If conversion is skipped, return here
	if rm.skipConversion {
		log.Printf("Skipping MP4 conversion...")
		return status, nil
	}

	// Convert .h264 to MP4 using ffmpeg (blocks while mutex is held)
	log.Printf("Converting to MP4...")
	if err := rm.convertToMP4(); err != nil {
		log.Printf("Warning: MP4 conversion failed: %v (raw .h264 preserved)", err)
		// Keep the .h264 file if conversion fails
	} else {
		// Conversion successful, delete the .h264 file
		os.Remove(rm.finalH264Path)
		log.Printf("MP4 finalized: %s", filepath.Base(rm.filePath))

		// Write metadata file
		meta := RecordingMeta{
			DurationMs: status.DurationMs,
			SizeBytes:  status.BytesWritten,
		}
		metaPath := rm.filePath + ".meta"
		if metaData, err := json.Marshal(meta); err == nil {
			os.WriteFile(metaPath, metaData, 0644)
		}
	}

	return status, nil
}

// convertToMP4 converts the raw .h264 file to MP4 using ffmpeg
func (rm *RecorderManager) convertToMP4() error {
	cmd := exec.Command("ffmpeg",
		"-f", "h264",
		"-i", rm.finalH264Path,
		"-c:v", "copy",
		"-movflags", "+faststart",
		"-y",
		rm.filePath,
	)

	// Capture output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// GetStatus returns current recording status
func (rm *RecorderManager) GetStatus() *RecordingStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.getStatusLocked()
}

func (rm *RecorderManager) getStatusLocked() *RecordingStatus {
	status := &RecordingStatus{
		Available:     true,
		Recording:     rm.recording.Load(),
		Finalizing:    rm.finalizing.Load(),
		MaxDurationMs: rm.maxDuration.Milliseconds(),
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
	// Cancel auto-stop timer if running
	if rm.stopTimer != nil {
		rm.stopTimer.Stop()
		rm.stopTimer = nil
	}
	// If recording is in progress, flush and close the file
	if rm.recording.Load() {
		rm.recording.Store(false)

		if rm.writer != nil {
			rm.writer.Flush()
			rm.writer = nil
		}
		if rm.file != nil {
			rm.file.Sync()
			rm.file.Close()
			rm.file = nil
		}
		log.Printf("Recording aborted during shutdown: %s", rm.tempH264Path)
	}
	rm.mu.Unlock()

	close(rm.naluChan)
}
