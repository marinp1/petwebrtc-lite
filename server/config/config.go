package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	Addr                       int
	Width                      int
	Height                     int
	Framerate                  int
	Rotation                   int
	Bitrate                    int // Optional: H264 bitrate in bits/sec (e.g., 1000000 = 1Mbps). If 0, rpicam-vid chooses automatically.
	CorsOrigin                 string
	RecordingDir               string // Optional: directory for recording files (must exist and be writable)
	RecordingUnavailableReason string // Reason why recording is unavailable (if RecordingDir is empty)
	RecordingSkipConversion    bool   // Optional, if ffmpeg finalisation should be ignored
}

// ParseConfig loads configuration from the given file path (TOML-like, key=value per line).
// If camera_cmd is not set, it is auto-generated from width, height, framerate, and rotation.
func ParseConfig(path string) *ServerConfig {
	// Defaults
	conf := &ServerConfig{
		Addr:                    8765,
		Width:                   1280,
		Height:                  720,
		Framerate:               30,
		Rotation:                180,
		CorsOrigin:              "*",
		RecordingSkipConversion: false,
	}

	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip surrounding quotes if present
			if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
			switch key {
			case "addr":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Addr = v
				}
			case "width":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Width = v
				}
			case "height":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Height = v
				}
			case "framerate":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Framerate = v
				}
			case "rotation":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Rotation = v
				}
			case "bitrate":
				if v, err := strconv.Atoi(val); err == nil {
					conf.Bitrate = v
				}
			case "cors_origin":
				conf.CorsOrigin = val
			case "recording_dir":
				conf.RecordingDir = val
			case "recording_skip_conversion":
				conf.RecordingSkipConversion = val == "true"
			}
		}
	}

	// Validate and fix invalid values
	conf.Validate()

	return conf
}

// checkFFmpegAvailable checks if ffmpeg is available in PATH
func checkFFmpegAvailable(c *ServerConfig) error {
	if c.RecordingSkipConversion == true {
		return nil
	}
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH (required for MP4 recording)")
	}
	return nil
}

// Validate checks configuration values and applies corrections or warnings
func (c *ServerConfig) Validate() {
	// Validate port range
	if c.Addr < 1 || c.Addr > 65535 {
		log.Printf("WARNING: Invalid port %d, using default 8765", c.Addr)
		c.Addr = 8765
	}

	// Validate dimensions
	if c.Width <= 0 {
		log.Printf("WARNING: Invalid width %d, using default 1280", c.Width)
		c.Width = 1280
	}
	if c.Height <= 0 {
		log.Printf("WARNING: Invalid height %d, using default 720", c.Height)
		c.Height = 720
	}

	// Validate framerate
	if c.Framerate <= 0 || c.Framerate > 120 {
		log.Printf("WARNING: Invalid framerate %d, using default 30", c.Framerate)
		c.Framerate = 30
	}

	// Validate rotation (must be 0, 90, 180, or 270)
	validRotations := map[int]bool{0: true, 90: true, 180: true, 270: true}
	if !validRotations[c.Rotation] {
		log.Printf("WARNING: Invalid rotation %d, using default 180", c.Rotation)
		c.Rotation = 180
	}

	// Warn about insecure CORS setting
	if c.CorsOrigin == "*" {
		log.Println("WARNING: CORS origin set to '*' - this is insecure for production")
	}

	// Validate recording directory if set
	if c.RecordingDir != "" {
		c.validateRecordingDir()
	} else {
		// No recording directory configured
		c.RecordingUnavailableReason = "No recording_dir configured"
	}
}

// validateRecordingDir validates the recording directory, retrying if the directory
// is not yet accessible (e.g. NFS mount not ready at boot). Retries up to 5 times
// with 2-second intervals before giving up.
func (c *ServerConfig) validateRecordingDir() {
	const maxRetries = 5
	const retryInterval = 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		reason := c.tryRecordingDir()
		if reason == "" {
			log.Printf("Recording enabled: %s (using ffmpeg for MP4 muxing)", c.RecordingDir)
			c.RecordingUnavailableReason = ""
			return
		}

		if attempt < maxRetries {
			log.Printf("Recording directory not ready (attempt %d/%d): %s — retrying in %v", attempt, maxRetries, reason, retryInterval)
			time.Sleep(retryInterval)
		} else {
			log.Printf("WARNING: Recording directory not ready after %d attempts: %s", maxRetries, reason)
			c.RecordingUnavailableReason = reason
			c.RecordingDir = "" // Disable recording
		}
	}
}

// tryRecordingDir checks if the recording directory is accessible and writable.
// Returns an empty string on success, or a reason string on failure.
func (c *ServerConfig) tryRecordingDir() string {
	info, err := os.Stat(c.RecordingDir)
	if err != nil {
		return fmt.Sprintf("Directory does not exist or is not accessible: %v", err)
	}
	if !info.IsDir() {
		return "Path is not a directory"
	}

	// Test if writable by creating a temp file
	testFile := c.RecordingDir + "/.write_test"
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Sprintf("Directory is not writable: %v", err)
	}
	f.Close()
	os.Remove(testFile)

	// Check if ffmpeg is available (required for MP4 muxing)
	if err := checkFFmpegAvailable(c); err != nil {
		return fmt.Sprintf("ffmpeg not available: %v", err)
	}

	return ""
}

// String returns a formatted string representation of the config for logging
func (c *ServerConfig) String() string {
	recording := "disabled"
	if c.RecordingDir != "" {
		recording = c.RecordingDir
	}
	bitrate := "auto"
	if c.Bitrate > 0 {
		bitrate = fmt.Sprintf("%dkbps", c.Bitrate/1000)
	}
	return fmt.Sprintf("Port=%d, Resolution=%dx%d@%dfps, Rotation=%d°, Bitrate=%s, CORS=%s, Recording=%s",
		c.Addr, c.Width, c.Height, c.Framerate, c.Rotation, bitrate, c.CorsOrigin, recording)
}
