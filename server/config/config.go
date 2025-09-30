package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ServerConfig struct {
	Addr      int
	CameraCmd string
	Width     int
	Height    int
	Framerate int
	Rotation  int
}

// ParseConfig loads configuration from the given file path (TOML-like, key=value per line).
// If camera_cmd is not set, it is auto-generated from width, height, framerate, and rotation.
func ParseConfig(path string) *ServerConfig {
	// Defaults
	conf := &ServerConfig{
		Addr:      8765,
		Width:     1280,
		Height:    720,
		Framerate: 30,
		Rotation:  180,
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
			case "camera_cmd":
				conf.CameraCmd = strings.Trim(val, "\"")
			}
		}
	}

	// Auto-generate camera command if not set
	if conf.CameraCmd == "" {
		conf.CameraCmd = fmt.Sprintf(
			"rpicam-vid -t 0 --width %d --height %d --framerate %d --inline --rotation %d --codec h264 --nopreview -o -",
			conf.Width, conf.Height, conf.Framerate, conf.Rotation,
		)
	}
	return conf
}
