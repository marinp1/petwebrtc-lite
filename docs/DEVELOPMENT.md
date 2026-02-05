# Development Guide

This guide covers building, developing, and testing PetWebRTC locally.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Building](#building)
- [Local Development](#local-development)
- [API Reference](#api-reference)
- [Project Structure](#project-structure)
- [Dependencies](#dependencies)

## Prerequisites

- **Go 1.23+** for server development
- **Node.js 18+** for client development
- **rpicam-vid** for camera streaming (Raspberry Pi only)

## Building

### Server (Go)

```bash
cd server

# Build for local architecture
go build -o ../builds/server-local .

# Cross-compile for Raspberry Pi
./scripts/build.sh
```

Output binaries:
- `builds/server-local` - Current architecture
- `builds/server-arm64` - Raspberry Pi 3/4/5 (64-bit)
- `builds/server-arm32` - Raspberry Pi Zero 2W (32-bit)

### Client (TypeScript/Vite)

```bash
cd client

# Install dependencies
npm install

# Build for production
npm run build

# Format code
npm run format
```

Production build outputs to `client/dist/`.

## Local Development

### Server

```bash
cd server
go run . config/server.conf
```

Server runs on `http://localhost:8765` by default.

### Client

```bash
cd client

# Start dev server with backend proxy
VITE_PROXY_TARGET=http://localhost:8765 npm run dev
```

Dev server runs at `http://localhost:5173` and proxies API requests to the Go server.

### Full Stack Development

1. Start the server: `cd server && go run . config/server.conf`
2. Start the client: `cd client && VITE_PROXY_TARGET=http://localhost:8765 npm run dev`
3. Open `http://localhost:5173` in your browser

## API Reference

### WebRTC Signaling

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/offer` | POST | Accept WebRTC SDP offer, return SDP answer |
| `/cameras` | GET | Return camera count for multi-camera setups |
| `/status` | GET | Server health check |

### Recording

Available when `recording_dir` is configured in `server.conf`.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/record/status` | GET | Get current recording status and duration |
| `/record/start` | POST | Start H264 recording |
| `/record/stop` | POST | Stop recording and save file |
| `/record/list` | GET | List all recordings with metadata |
| `/record/download/{filename}` | GET | Download a recording file |

### Example: WebRTC Offer

```bash
curl -X POST http://localhost:8765/offer \
  -H "Content-Type: application/json" \
  -d '{"sdp": "...", "type": "offer"}'
```

Response:
```json
{
  "sdp": "v=0\r\n...",
  "type": "answer"
}
```

## Project Structure

```
.
├── server/                 # Go server
│   ├── main.go            # HTTP server, signaling endpoint
│   ├── internal/
│   │   ├── camera.go      # Camera process management, H264 parsing
│   │   ├── media.go       # Client manager, RTP packetization
│   │   ├── signaling.go   # WebRTC offer/answer exchange
│   │   ├── recorder.go    # H264 recording to disk
│   │   └── recording_handlers.go
│   └── config/            # Configuration files
│
├── client/                # TypeScript/Vite web client
│   ├── src/
│   │   ├── main.ts        # Entry point, carousel setup
│   │   ├── carousel.ts    # Carousel controller
│   │   ├── connect.ts     # WebRTC connection management
│   │   ├── detector.ts    # MediaPipe object detection
│   │   ├── recording.ts   # Recording controls
│   │   ├── recordings-panel.ts
│   │   ├── gestures.ts    # Touch/swipe handling
│   │   ├── navigation.ts  # Dots and arrow navigation
│   │   └── styles/        # Modular CSS files
│   ├── vite.config.ts
│   └── index.html
│
├── converter/             # H264 to MP4 converter service
├── builds/                # Compiled server binaries
├── scripts/               # Build and deployment scripts
└── docs/                  # Documentation
```

## Dependencies

### Server (Go)

| Package | Version | Purpose |
|---------|---------|---------|
| [pion/webrtc](https://github.com/pion/webrtc) | v4.1.4 | WebRTC implementation |
| [pion/rtp](https://github.com/pion/rtp) | v1.8.21 | RTP packetization |

### Client (TypeScript)

| Package | Version | Purpose |
|---------|---------|---------|
| [@mediapipe/tasks-vision](https://www.npmjs.com/package/@mediapipe/tasks-vision) | v0.10.22-rc | Object detection |
| TypeScript | 5.8.3 | Type checking |
| Vite | latest | Build tool (Rolldown bundler) |
| Biome | 2.2.4 | Formatter/linter |

## Performance Notes

- **Large buffered reader** (256KB) minimizes syscalls when reading H264 stream
- **Non-blocking channel sends** drop frames when buffer fills instead of blocking
- **Keyframe caching** lets new clients start playback immediately
- **Lazy connection loading** only maintains WebRTC connections to visible cameras
- **Buffered writes** (64KB) reduce I/O overhead on Pi Zero 2 W
