package main

import (
	"log"
	"net/http"

	"github.com/pion/webrtc/v3"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

func main() {
	conf := config.ParseFlags()

	m := internal.SetupMediaEngine()
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	clients := internal.NewClientManager()

	go clients.StartCamera(conf.CameraCmd)
	go clients.BroadcastNALUs()

	http.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clients, conf)
	})

	log.Printf("WebRTC server running on %s", conf.Addr)
	log.Fatal(http.ListenAndServe(conf.Addr, nil))
}
