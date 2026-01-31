package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type ServerConfig struct {
	Addr         int
	Width        int
	Height       int
	Framerate    int
	Rotation     int
	CorsOrigin   string
	RecordingDir string // Optional: directory for recording files (must exist and be writable)
}

// ParseConfig loads configuration from the given file path (TOML-like, key=value per line).
// If camera_cmd is not set, it is auto-generated from width, height, framerate, and rotation.
func ParseConfig(path string) *ServerConfig {
	// Defaults
	conf := &ServerConfig{
		Addr:       8765,
		Width:      1280,
		Height:     720,
		Framerate:  30,
		Rotation:   180,
		CorsOrigin: "*",
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
			case "cors_origin":
				conf.CorsOrigin = val
			case "recording_dir":
				conf.RecordingDir = val
			}
		}
	}

	// Validate and fix invalid values
	conf.Validate()

	return conf
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
		info, err := os.Stat(c.RecordingDir)
		if err != nil {
			log.Printf("WARNING: Recording directory %q does not exist or is not accessible: %v", c.RecordingDir, err)
			c.RecordingDir = "" // Disable recording
		} else if !info.IsDir() {
			log.Printf("WARNING: Recording path %q is not a directory", c.RecordingDir)
			c.RecordingDir = "" // Disable recording
		} else {
			// Test if writable by creating a temp file
			testFile := c.RecordingDir + "/.write_test"
			if f, err := os.Create(testFile); err != nil {
				log.Printf("WARNING: Recording directory %q is not writable: %v", c.RecordingDir, err)
				c.RecordingDir = "" // Disable recording
			} else {
				f.Close()
				os.Remove(testFile)
				log.Printf("Recording enabled: %s", c.RecordingDir)
			}
		}
	}
}

// String returns a formatted string representation of the config for logging
func (c *ServerConfig) String() string {
	recording := "disabled"
	if c.RecordingDir != "" {
		recording = c.RecordingDir
	}
	return fmt.Sprintf("Port=%d, Resolution=%dx%d@%dfps, Rotation=%d°, CORS=%s, Recording=%s",
		c.Addr, c.Width, c.Height, c.Framerate, c.Rotation, c.CorsOrigin, recording)
}
