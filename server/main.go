package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/pion/webrtc/v4"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin; for production, restrict to your front-end URL
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
	// Load config from file (default: config/server.conf)
	conf := config.ParseConfig("config/server.conf")

	m := internal.SetupMediaEngine()
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	clients := internal.NewClientManager()

	go clients.StartCamera(conf.CameraCmd)
	go clients.BroadcastNALUs()

	http.Handle("/offer", enableCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clients, conf)
	})))

	port := fmt.Sprintf(":%d", conf.Addr)
	log.Printf("WebRTC server running on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
