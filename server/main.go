package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed public/*
var publicFS embed.FS

const (
	hlsDir      = "./hls"
	playlist    = "stream.m3u8"
	segmentTime = 1 // HLS segment length in seconds
)

func startHLS() error {
	absHLSDir, err := filepath.Abs(hlsDir)
	if err != nil {
		return err
	}

	// Ensure HLS directory exists
	if err := os.MkdirAll(absHLSDir, 0755); err != nil {
		return err
	}

	// rpicam-vid -> ffmpeg HLS pipeline
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(
			`rpicam-vid -t 0 --codec h264 --nopreview -o - | ffmpeg -f h264 -i - -c copy -f hls -hls_time %d -hls_list_size 5 -hls_flags delete_segments+append_list %s/%s`,
			segmentTime, absHLSDir, playlist,
		),
	)

	// Log ffmpeg output
	cmd.Stderr = log.Writer()

	return cmd.Start()
}

func main() {
	// Start HLS pipeline
	if err := startHLS(); err != nil {
		log.Fatal("Failed to start HLS pipeline:", err)
	}

	// Serve HLS segments
	http.Handle("/hls/", http.StripPrefix("/hls/", http.FileServer(http.Dir(hlsDir))))

	// Serve embedded public folder at root
	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(subFS)))

	fmt.Println("HLS server running on :8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}
