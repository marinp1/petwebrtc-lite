# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PetWebRTC-Lite is a lightweight WebRTC streaming server for live H264 video from Raspberry Pi cameras to web browsers. The server is written in Go and uses Pion WebRTC for peer connections. The client is a TypeScript/Vite application with optional MediaPipe object detection.

## Development Commands

### Client (TypeScript/Vite)
```bash
cd client
npm run format              # Format code with Biome
npm run dev                 # Start dev server (localhost:5173)
npm run build               # TypeScript compile + Vite bundle → dist/
npm run preview             # Preview production build

# Dev server with backend proxy (backend must be running separately)
VITE_PROXY_TARGET=http://localhost:8765 npm run dev
```

### Server (Go)
```bash
cd server
go build -o ../builds/server-local .        # Local build
go run . config/server.conf                 # Run directly

# Cross-compile for Raspberry Pi
./scripts/build.sh                          # Builds arm64 and arm32 to builds/
```

### Deployment
```bash
./scripts/deploy-server.sh deploy ipcam            # Deploy binary only
./scripts/deploy-server.sh deploy-start ipcam      # Deploy and start
./scripts/deploy-client.sh                         # Deploy client assets
```

## Architecture

### Data Flow
1. **Camera → Server**: `rpicam-vid` outputs H264 byte stream → [CameraManager](server/internal/camera.go) parses NAL units (SPS, PPS, IDR, P-frames) → broadcasts on channel
2. **Server → Clients**: [ClientManager](server/internal/media.go) caches keyframes (SPS/PPS/IDR) and broadcasts NAL units to all connected WebRTC clients
3. **RTP Packetization**: Each client has a Pion RTP H264Packetizer (MTU=1200) that converts NAL units to RTP packets
4. **WebRTC Signaling**: Browser POSTs SDP offer to `/offer` → [server](server/internal/signaling.go) creates answer → peer connection established
5. **Client Rendering**: [connect.ts](client/src/connect.ts) receives video track → displays in `<video>` element → optional [detector.ts](client/src/detector.ts) runs MediaPipe on canvas

### Key Design Decisions

**Performance Optimizations** ([camera.go:1-30](server/internal/camera.go#L1-L30)):
- Large buffered reader (256KB) to minimize syscalls when reading H264 stream
- Non-blocking channel sends: drops frames when buffer fills instead of blocking camera process
- Efficient NAL unit parsing to avoid reprocessing

**Broadcasting** ([media.go](server/internal/media.go)):
- Caches latest SPS/PPS/IDR keyframes so new clients can start decoding immediately
- Non-blocking broadcast to all clients with per-client buffered channels (500 NAL units)
- Per-client NALU channels tolerate bursts without blocking other clients

**Client Architecture**:
- [main.ts](client/src/main.ts): Detects camera count from `/cameras` endpoint, creates grid layout
- [connect.ts](client/src/connect.ts): Manages WebRTC peer connection, receives stats via data channel, updates connection badges
- [detector.ts](client/src/detector.ts): MediaPipe object detection with DPI-aware canvas overlay, draws detections as Catmull-Rom splines
- [storage.ts](client/src/storage.ts): Persists detection toggle state to localStorage

## Key Files by Function

### Server (Go)
- [server/main.go](server/main.go) - HTTP server setup, serves static client files, `/offer` signaling endpoint
- [server/internal/camera.go](server/internal/camera.go) - Camera process lifecycle, H264 NAL parsing, kills stale processes
- [server/internal/media.go](server/internal/media.go) - Client manager, RTP packetization, keyframe caching
- [server/internal/signaling.go](server/internal/signaling.go) - WebRTC offer/answer exchange, ICE gathering
- [server/config/server.conf](server/config/server.conf) - Camera settings (resolution, FPS, rotation)

### Client (TypeScript)
- [client/src/main.ts](client/src/main.ts) - Entry point, multi-camera grid setup
- [client/src/connect.ts](client/src/connect.ts) - WebRTC connection, stream handling, badge updates
- [client/src/detector.ts](client/src/detector.ts) - MediaPipe object detection, canvas visualization
- [client/vite.config.ts](client/vite.config.ts) - Dev proxy config, WASM module copying

## Configuration

Edit [server/config/server.conf](server/config/server.conf):
```ini
addr = 8765               # Server port
width = 1280              # Camera resolution
height = 720
framerate = 30
rotation = 180            # Camera rotation (0, 90, 180, 270)
```

## Testing Locally

1. Start server: `cd server && go run . config/server.conf`
2. Start client dev server: `cd client && VITE_PROXY_TARGET=http://localhost:8765 npm run dev`
3. Open browser to `http://localhost:5173`

## Common Tasks

### Add Client Feature
1. Edit TypeScript files in [client/src/](client/src/)
2. Test with `npm run dev`
3. Build production: `npm run build`
4. Deploy: `./scripts/deploy-client.sh`

### Modify Server Behavior
1. Edit Go files in [server/internal/](server/internal/)
2. Test locally: `go run . config/server.conf`
3. Cross-compile: `./scripts/build.sh`
4. Deploy: `./scripts/deploy-server.sh deploy-start ipcam`

### Debug WebRTC Connection
- Check browser console for WebRTC state transitions (in [connect.ts](client/src/connect.ts))
- Server logs show ICE gathering and peer connection events
- Data channel sends stats (dropped frames, timestamps) from client to server

## Dependencies

**Server**: Pion WebRTC v4.1.4, Pion RTP v1.8.21, Go 1.23.11
**Client**: MediaPipe Vision v0.10.22-rc, TypeScript 5.8.3, Vite (Rolldown), Biome 2.2.4

## Security Note

Server has no authentication. Use behind a reverse proxy (nginx, Caddy) on untrusted networks. WebRTC uses DTLS-SRTP for media encryption.
