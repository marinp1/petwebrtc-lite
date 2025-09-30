package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pion/webrtc/v4"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

func enableCORS(corsOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin; for production, restrict to your front-end URL
		w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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

	clients := internal.NewClientManager()

	cameraCmd := fmt.Sprintf(
		"rpicam-vid -t 0 --width %d --height %d --framerate %d --inline --rotation %d --codec h264 --nopreview -o -",
		conf.Width, conf.Height, conf.Framerate, conf.Rotation,
	)

	go clients.StartCamera(cameraCmd)
	go clients.BroadcastNALUs()

	http.Handle("/status", enableCORS(conf.CorsOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})))

	http.Handle("/offer", enableCORS(conf.CorsOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clients, conf)
	})))

	port := fmt.Sprintf(":%d", conf.Addr)
	log.Printf("WebRTC server running on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
