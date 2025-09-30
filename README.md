
# PetWebRTC-Lite

PetWebRTC-Lite is a lightweight, headless WebRTC streaming server and client for live H264 video streaming from a Raspberry Pi camera (or similar) to web browsers. It is designed for low-latency, real-time video delivery using the [Pion WebRTC](https://github.com/pion/webrtc) library.

## Features

- Streams H264 video from a camera to any modern web browser using WebRTC
- Minimal dependencies, runs on Raspberry Pi and ARM devices
- Simple web client for viewing the stream
- Configurable camera parameters (resolution, framerate, rotation, etc.)
- Easy deployment and cross-compilation

## Architecture

- **Server** (`server/`): Go application that manages camera process, parses H264, handles WebRTC signaling, and streams video to clients.
- **Client** (`client/index.html`): HTML/JS web client that connects to the server via WebRTC and displays the video stream.
- **Config** (`server/config/server.conf`): Simple config file for camera and server settings.
- **Build/Deploy Scripts** (`scripts/`): Shell scripts for building and deploying the server binary.

## Getting Started

### Prerequisites

- Go 1.18+ (for building the server)
- Raspberry Pi OS or Linux (for camera streaming)
- [rpicam-vid](https://www.raspberrypi.com/documentation/computers/camera_software.html)

### Build the Server

```bash
./scripts/build.sh
```
This builds the ARM64 server binary at `builds/server-arm64`.

### Deploy to Raspberry Pi (or remote host)

Edit `scripts/deploy.sh` to set your `REMOTE_HOST` and `REMOTE_DIR`.

```bash
./scripts/deploy.sh deploy-start
```
This stops any running instance, copies the binary, and starts the server in the background.

### Set service

Create service file `nano /etc/systemd/system/petwebrtc.service` with the following contents:

```
[Unit]
Description=PetWebRTC Server
After=network.target

[Service]
ExecStart=/home/<username>/opt/bin/ipcam/petwebrtc-arm<architecture>
WorkingDirectory=/home/<username>/opt/bin/ipcam
Restart=always
User=<username>
Group=<username>

# Discard all logs
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
```

Enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable petwebrtc.service
sudo systemctl start petwebrtc.service
# check status
systemctl status petwebrtc.service
```

### Web Client

Open `client/index.html` in your browser. By default, it connects to `/offer` on the same host.

## Configuration

Edit `server/config/server.conf` to set camera and server parameters:

```
addr = 8765
width = 1280
height = 720
framerate = 30
rotation = 180
# camera_cmd = "rpicam-vid ..."
```

## API Endpoints

- `POST /offer`: Accepts a WebRTC SDP offer and returns an SDP answer (used by the web client).

## Dependencies

- [Pion WebRTC](https://github.com/pion/webrtc)
- [Pion RTP](https://github.com/pion/rtp)
- `rpicam-vid`

## Security

This server is intended for use on trusted networks or behind a secure proxy. It does not implement authentication or encryption beyond what is provided by WebRTC/STUN.

## Project Structure

- `server/` — Go server source code
- `client/` — Web client (HTML/JS)
- `builds/` — Compiled server binaries
- `scripts/` — Build and deployment scripts
- `server/config/` — Configuration files

## License

MIT License