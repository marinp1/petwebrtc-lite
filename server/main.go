package main

import (
	"io/fs"
	"log"
	"net/http"
	"os/exec"
)

func main() {
	// Kill previous camera/ffmpeg processes
	exec.Command("pkill", "-f", "rpicam-vid").Run()
	exec.Command("pkill", "-f", "ffmpeg").Run()

	publicFS, err := fs.Sub(content, "public")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(publicFS)))
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("Server running at http://0.0.0.0:8765")
	log.Fatal(http.ListenAndServe(":8765", nil))
}
