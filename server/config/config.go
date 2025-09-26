package config

import (
	"flag"
	"fmt"
)

type ServerConfig struct {
	Addr      string
	CameraCmd string
	Width     int
	Height    int
	Framerate int
	Rotation  int
	Codec     string
	Preview   bool
}

func ParseFlags() *ServerConfig {
	addr := flag.String("addr", ":8765", "HTTP server address")
	width := flag.Int("width", 1280, "Camera width")
	height := flag.Int("height", 720, "Camera height")
	framerate := flag.Int("framerate", 30, "Camera framerate")
	rotation := flag.Int("rotation", 180, "Camera rotation")
	codec := flag.String("codec", "h264", "Camera codec")
	preview := flag.Bool("preview", false, "Show camera preview")
	flag.Parse()

	nopreview := ""
	if !*preview {
		nopreview = " --nopreview"
	}
	cmd := fmt.Sprintf(
		"rpicam-vid -t 0 --width %d --height %d --framerate %d --inline --rotation %d --codec %s%s -o -",
		*width, *height, *framerate, *rotation, *codec, nopreview,
	)

	return &ServerConfig{
		Addr:      *addr,
		CameraCmd: cmd,
		Width:     *width,
		Height:    *height,
		Framerate: *framerate,
		Rotation:  *rotation,
		Codec:     *codec,
		Preview:   *preview,
	}
}
