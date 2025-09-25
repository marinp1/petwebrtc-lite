package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
)

const (
	hlsDir      = "./hls"       // HLS segments directory
	playlist    = "stream.m3u8" // playlist filename
	segmentTime = 1             // HLS segment length in seconds
)

func startHLS() error {
	// Create HLS directory if it doesn't exist
	_, err := filepath.Abs(hlsDir)
	if err != nil {
		return err
	}

	// Run rpicam-vid piped into ffmpeg to generate HLS segments
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(
			`rpicam-vid -t 0 --codec h264 --nopreview -o - | ffmpeg -i - -c copy -f hls -hls_time %d -hls_list_size 5 -hls_flags delete_segments %s/%s`,
			segmentTime, hlsDir, playlist,
		),
	)

	// Optional: redirect stderr for debugging
	cmd.Stderr = log.Writer()

	return cmd.Start() // run in background
}

func main() {
	// Start HLS generation
	if err := startHLS(); err != nil {
		log.Fatal("Failed to start HLS pipeline:", err)
	}

	// Serve HLS directory
	http.Handle("/hls/", http.StripPrefix("/hls/", http.FileServer(http.Dir(hlsDir))))

	// HTML page with <video> tag
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<title>Pi Camera HLS Stream</title>
		</head>
		<body>
			<h2>Raspberry Pi Camera HLS Stream</h2>
			<video src="/hls/stream.m3u8" width="640" height="480" autoplay muted playsinline controls></video>
		</body>
		</html>`
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	})

	fmt.Println("HLS server running on :8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}
