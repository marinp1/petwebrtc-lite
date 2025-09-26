package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/pion/webrtc/v3"

	"webrtc-ipcam/config"
	"webrtc-ipcam/internal"
)

//go:embed public/*
var publicFS embed.FS

func main() {
	conf := config.ParseFlags()

	m := internal.SetupMediaEngine()
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	subFS, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(subFS)))

	clients := internal.NewClientManager()

	go clients.StartCamera(conf.CameraCmd)
	go clients.BroadcastNALUs()

	http.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		internal.HandleOffer(w, r, api, clients, conf)
	})

	log.Printf("WebRTC server running on %s", conf.Addr)
	log.Fatal(http.ListenAndServe(conf.Addr, nil))
}
