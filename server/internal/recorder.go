package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// RecorderManager handles H264 recording to file
type RecorderManager struct {
	mu            sync.RWMutex
	recording     atomic.Bool
	file          *os.File
	filePath      string
	startTime     time.Time
	bytesWritten  int64
	framesWritten int64
	recordingDir  string
	naluChan      chan []byte
	done          chan struct{}
	wg            sync.WaitGroup

	// Cached keyframes for starting recordings
	lastSPS []byte
	lastPPS []byte
}

// RecordingStatus represents the current recording state
type RecordingStatus struct {
	Available     bool   `json:"available"`
	Recording     bool   `json:"recording"`
	FilePath      string `json:"filePath,omitempty"`
	StartTime     int64  `json:"startTime,omitempty"`
	DurationMs    int64  `json:"durationMs,omitempty"`
	BytesWritten  int64  `json:"bytesWritten,omitempty"`
	FramesWritten int64  `json:"framesWritten,omitempty"`
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

// Start begins recording to a new file
func (rm *RecorderManager) Start() (*RecordingStatus, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.recording.Load() {
		return nil, fmt.Errorf("recording already in progress")
	}

	// Generate filename with timestamp: recording_20260131_143052.h264
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("recording_%s.h264", timestamp)
	rm.filePath = filepath.Join(rm.recordingDir, filename)

	file, err := os.Create(rm.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	rm.file = file
	rm.startTime = time.Now()
	rm.bytesWritten = 0
	rm.framesWritten = 0

	// Write cached SPS/PPS first (required for decodable stream)
	if rm.lastSPS != nil {
		n, _ := rm.file.Write(rm.lastSPS)
		rm.bytesWritten += int64(n)
	}
	if rm.lastPPS != nil {
		n, _ := rm.file.Write(rm.lastPPS)
		rm.bytesWritten += int64(n)
	}

	rm.recording.Store(true)

	return rm.getStatusLocked(), nil
}

// Stop ends the current recording
func (rm *RecorderManager) Stop() (*RecordingStatus, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.recording.Load() {
		return nil, fmt.Errorf("no recording in progress")
	}

	rm.recording.Store(false)

	status := rm.getStatusLocked()

	if rm.file != nil {
		rm.file.Sync()
		rm.file.Close()

		// Write metadata file
		meta := RecordingMeta{
			DurationMs: status.DurationMs,
			SizeBytes:  status.BytesWritten,
		}
		metaPath := rm.filePath + ".meta"
		if metaData, err := json.Marshal(meta); err == nil {
			os.WriteFile(metaPath, metaData, 0644)
		}

		rm.file = nil
	}

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
	// Cache SPS/PPS for starting new recordings
	if len(nalu) >= 5 {
		naluType := nalu[4] & 0x1F
		switch naluType {
		case 7: // SPS
			rm.mu.Lock()
			rm.lastSPS = make([]byte, len(nalu))
			copy(rm.lastSPS, nalu)
			rm.mu.Unlock()
		case 8: // PPS
			rm.mu.Lock()
			rm.lastPPS = make([]byte, len(nalu))
			copy(rm.lastPPS, nalu)
			rm.mu.Unlock()
		}
	}

	// Write to file if recording
	if rm.recording.Load() {
		rm.mu.Lock()
		if rm.file != nil {
			n, err := rm.file.Write(nalu)
			if err == nil {
				rm.bytesWritten += int64(n)
				rm.framesWritten++
			}
		}
		rm.mu.Unlock()
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
		// Only include .h264 files
		if filepath.Ext(name) != ".h264" {
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
	if filepath.Ext(filename) != ".h264" {
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
	if rm.file != nil {
		rm.file.Sync()
		rm.file.Close()
		rm.file = nil
	}
	rm.mu.Unlock()

	close(rm.naluChan)
}
