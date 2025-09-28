
# WebRTC-IPCam Server Documentation

## Overview

This application is a lightweight WebRTC streaming server designed to stream live H264 video from a Raspberry Pi camera (or similar) to web clients using the WebRTC protocol. It leverages the [Pion WebRTC](https://github.com/pion/webrtc) library for real-time media transport and signaling, and can be run headless on devices like the Raspberry Pi.

## Architecture

The server consists of three main components:

- **Configuration (`config/config.go`)**: Parses command-line flags to configure camera parameters (resolution, framerate, rotation, etc.) and builds the shell command to launch the camera streaming process (e.g., `rpicam-vid`).
- **WebRTC Core (`internal/webrtc.go`)**: Implements the WebRTC signaling, manages peer connections, handles H264 NALU packetization, and broadcasts video to all connected clients. It also manages the camera process lifecycle.
- **Server Entrypoint (`main.go`)**: Initializes configuration, sets up the WebRTC API, starts the camera and NALU broadcaster goroutines, and exposes the `/offer` HTTP endpoint for WebRTC signaling.

## Main Files

- `main.go`: Application entrypoint. Sets up configuration, WebRTC API, client manager, and HTTP server.
- `config/config.go`: Defines `ServerConfig` and parses command-line flags for camera and server settings.
- `internal/webrtc.go`: Core WebRTC logic, including:
	- `Client` and `ClientManager` types for managing connections and video distribution
	- Camera process management (start/stop)
	- H264 NALU parsing and RTP packetization
	- WebRTC offer/answer handling and ICE negotiation

## How It Works

1. **Startup**: The server parses command-line flags to determine camera and server settings, then constructs the camera command (default: `rpicam-vid ...`).
2. **Camera Streaming**: The camera process is started, outputting H264 video to stdout. The server reads this stream, extracts H264 NAL units, and queues them for broadcast.
3. **WebRTC Signaling**: Clients POST an SDP offer to `/offer`. The server creates a new WebRTC peer connection, adds a video track, and responds with an SDP answer.
4. **Video Broadcast**: H264 NAL units are packetized into RTP and sent to all connected clients with active WebRTC sessions.
5. **Client Management**: Disconnected or failed clients are automatically removed.

## Configuration Flags

- `-addr` (default: 8765): HTTP server port
- `-width` (default: 1280): Camera video width
- `-height` (default: 720): Camera video height
- `-framerate` (default: 30): Camera framerate
- `-rotation` (default: 180): Camera rotation

## Endpoints

- `POST /offer`: Accepts a WebRTC SDP offer from a client and returns an SDP answer. Used for establishing the WebRTC connection.

## Dependencies

- [Pion WebRTC](https://github.com/pion/webrtc)
- [Pion RTP](https://github.com/pion/rtp)
- `rpicam-vid` or `ffmpeg` (for camera streaming)

## Example Usage

```sh
go run main.go -width 1920 -height 1080 -framerate 30 -rotation 0
```

## Security Note

This server is intended for use on trusted networks or behind a secure proxy. It does not implement authentication or encryption beyond what is provided by WebRTC/STUN.

---

For more details, see the inline documentation in each Go source file.
