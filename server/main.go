package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pion/webrtc/v4"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

func enableCORS(corsOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin; for production, restrict to your front-end URL
		w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load config from file beside the running binary (optional)
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	confPath := filepath.Join(filepath.Dir(execPath), "server.conf")
	conf := config.ParseConfig(confPath)

	m := internal.SetupMediaEngine()
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	config := internal.CameraConfig{
		ChannelBuffer: 2000,       // Handle bursts
		ReadBuffer:    256 * 1024, // 256KB reads
	}

	cameraManager := internal.NewCameraManager(config)
	clientManager := internal.NewClientManager()

	cameraCmd := fmt.Sprintf(
		"rpicam-vid -t 0 --width %d --height %d --framerate %d --inline --rotation %d --codec h264 --nopreview -o -",
		conf.Width, conf.Height, conf.Framerate, conf.Rotation,
	)

	if err := cameraManager.StartCamera(cameraCmd); err != nil {
		log.Fatalf("Failed to start camera: %v", err)
	}
	go clientManager.BroadcastNALUs(cameraManager.GetNALUChannel())

	http.Handle("/status", enableCORS(conf.CorsOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write status response: %v", err)
		}
	})))

	http.Handle("/offer", enableCORS(conf.CorsOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clientManager, conf)
	})))

	port := fmt.Sprintf(":%d", conf.Addr)
	server := &http.Server{
		Addr: port,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Printf("WebRTC server running on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutdown signal received, cleaning up...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Stop camera
	if err := cameraManager.Stop(); err != nil {
		log.Printf("Camera stop error: %v", err)
	}

	// Close all peer connections and wait for cleanup
	clientManager.Mu.Lock()
	clients := make([]*internal.Client, 0, len(clientManager.Clients))
	for c := range clientManager.Clients {
		clients = append(clients, c)
	}
	clientManager.Mu.Unlock()

	for _, c := range clients {
		c.PeerConn.Close()
		clientManager.RemoveClient(c)
	}

	log.Println("Server shut down cleanly.")
}
