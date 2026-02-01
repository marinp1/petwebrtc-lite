package internal

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// HandleRecordStatus handles GET /record/status
func HandleRecordStatus(w http.ResponseWriter, r *http.Request, recorder *RecorderManager, unavailableReason string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var status *RecordingStatus
	if recorder != nil {
		status = recorder.GetStatus()
	} else {
		status = &RecordingStatus{
			Available:         false,
			Recording:         false,
			UnavailableReason: unavailableReason,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleRecordStart handles POST /record/start
func HandleRecordStart(w http.ResponseWriter, r *http.Request, recorder *RecorderManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if recorder == nil {
		http.Error(w, "recording not available", http.StatusServiceUnavailable)
		return
	}

	status, err := recorder.Start()
	if err != nil {
		log.Printf("Failed to start recording: %v", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	log.Printf("Recording started: %s", status.FilePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleRecordStop handles POST /record/stop
func HandleRecordStop(w http.ResponseWriter, r *http.Request, recorder *RecorderManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if recorder == nil {
		http.Error(w, "recording not available", http.StatusServiceUnavailable)
		return
	}

	status, err := recorder.Stop()
	if err != nil {
		log.Printf("Failed to stop recording: %v", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	log.Printf("Recording stopped: %s (duration: %dms, size: %d bytes)",
		status.FilePath, status.DurationMs, status.BytesWritten)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleRecordList handles GET /record/list
func HandleRecordList(w http.ResponseWriter, r *http.Request, recorder *RecorderManager) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if recorder == nil {
		http.Error(w, "recording not available", http.StatusServiceUnavailable)
		return
	}

	recordings, err := recorder.ListRecordings()
	if err != nil {
		log.Printf("Failed to list recordings: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Recordings []RecordingFile `json:"recordings"`
	}{
		Recordings: recordings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRecordDownload handles GET /record/download/{filename}
func HandleRecordDownload(w http.ResponseWriter, r *http.Request, recorder *RecorderManager) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if recorder == nil {
		http.Error(w, "recording not available", http.StatusServiceUnavailable)
		return
	}

	// Extract filename from path: /record/download/{filename}
	path := r.URL.Path
	prefix := "/record/download/"
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	filename := strings.TrimPrefix(path, prefix)

	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	filePath, err := recorder.GetFilePath(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for Content-Length
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to stat file", http.StatusInternalServerError)
		return
	}

	// Use application/octet-stream to prevent browser manipulation
	// VLC and other players will recognize .h264 extension
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	io.Copy(w, file)
}
