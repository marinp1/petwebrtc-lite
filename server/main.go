package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed public/*
var publicFS embed.FS

const (
	hlsSubDir   = "hls"         // HLS folder relative to binary
	playlist    = "stream.m3u8" // HLS playlist name
	segmentTime = 1             // HLS segment length in seconds
)

func startHLS() (*exec.Cmd, *exec.Cmd, error) {
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve binary dir: %w", err)
	}
	absHLSDir := filepath.Join(exeDir, hlsSubDir)

	if err := os.MkdirAll(absHLSDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create HLS dir: %w", err)
	}

	// rpicam-vid command
	rpicam := exec.Command(
		"rpicam-vid",
		"-t", "0",
		"--codec", "h264",
		"--nopreview",
		"-o", "-",
	)

	// ffmpeg command
	ffmpeg := exec.Command(
		"ffmpeg",
		"-fflags", "+genpts",
		"-f", "h264",
		"-i", "pipe:0",
		"-c:v", "copy",
		"-f", "hls",
		"-hls_time", fmt.Sprint(segmentTime),
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+append_list",
		filepath.Join(absHLSDir, playlist),
	)

	// Pipe rpicam stdout -> ffmpeg stdin
	stdout, err := rpicam.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect rpicam stdout: %w", err)
	}
	ffmpeg.Stdin = stdout

	// Forward logs
	rpicam.Stderr = log.Writer()
	ffmpeg.Stderr = log.Writer()

	// Start both processes
	if err := rpicam.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start rpicam-vid: %w", err)
	}
	if err := ffmpeg.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	return rpicam, ffmpeg, nil
}

func main() {
	// Start HLS pipeline
	rpicam, ffmpeg, err := startHLS()
	if err != nil {
		log.Fatal("Failed to start HLS pipeline:", err)
	}

	// Prepare HTTP server
	exeDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	hlsDir := filepath.Join(exeDir, hlsSubDir)
	http.Handle("/hls/", http.StripPrefix("/hls/", http.FileServer(http.Dir(hlsDir))))

	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(subFS)))

	server := &http.Server{Addr: ":8765"}

	// Graceful shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("Shutting down server and stopping HLS processes...")
		rpicam.Process.Kill()
		ffmpeg.Process.Kill()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	fmt.Println("HLS server running on :8765")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
