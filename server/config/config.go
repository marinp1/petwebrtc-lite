package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type ServerConfig struct {
	Addr       int
	Width      int
	Height     int
	Framerate  int
	Rotation   int
	CorsOrigin string
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
			}
		}
	}

	return conf
}
