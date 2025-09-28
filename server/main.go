package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/pion/webrtc/v3"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

func main() {
	// Load config from file (default: config/server.conf)
	conf := config.ParseConfig("config/server.conf")

	m := internal.SetupMediaEngine()
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	clients := internal.NewClientManager()

	go clients.StartCamera(conf.CameraCmd)
	go clients.BroadcastNALUs()

	http.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clients, conf)
	})

	port := fmt.Sprintf(":%d", conf.Addr)
	log.Printf("WebRTC server running on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
