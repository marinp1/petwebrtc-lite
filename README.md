# PetWebRTC-Lite

Lightweight WebRTC streaming server for live H264 video from Raspberry Pi cameras to web browsers.

## Features

- **Low-latency WebRTC streaming** from Raspberry Pi camera to any modern browser
- **Multi-camera support** with carousel UI and swipe navigation
- **H264 recording** with on-demand start/stop and downloadable recordings
- **MediaPipe object detection** with canvas overlay (optional)
- **Mobile-first responsive design** with touch gestures and keyboard navigation
- **Connection health monitoring** with automatic reconnect and status indicators
- **Minimal footprint** - runs efficiently on Raspberry Pi Zero 2 W and up

## Architecture

```
Camera → rpicam-vid → [Go Server] → WebRTC → Browser
                          │
                          └──→ H264 Recording → NAS (optional)
```

- **Server** ([server/](server/)): Go + Pion WebRTC for signaling and streaming
- **Client** ([client/](client/)): TypeScript/Vite web app with carousel UI

## Quick Start

### Build

```bash
# Server (cross-compile for Pi)
cd server && ../scripts/build.sh

# Client
cd client && npm install && npm run build
```

### Deploy to Raspberry Pi

```bash
# Deploy server binary and start
./scripts/deploy-server.sh deploy-start <hostname>

# Deploy client assets
./scripts/deploy-client.sh
```

### Configure

Edit `server/config/server.conf`:

```ini
addr = 8765
width = 1280
height = 720
framerate = 30
rotation = 180
recording_dir = /mnt/nas  # Optional: enable recording
```

## Documentation

| Document | Description |
|----------|-------------|
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Full deployment guide: systemd services, NAS/NFS setup, H264 converter |
| [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | Building, local dev server, API reference, project structure |
| [converter/README.md](converter/README.md) | H264 to MP4 converter service |

## Client Features

- **Carousel UI** - Swipe navigation, keyboard shortcuts, lazy loading
- **Connection monitoring** - Status badges, health detection, auto-reconnect
- **Object detection** - MediaPipe Vision with DPI-aware canvas overlay
- **Recording controls** - Start/stop recording, view/download recordings

## Security

No built-in authentication. Use behind a reverse proxy (nginx, Caddy) on untrusted networks. WebRTC uses DTLS-SRTP for media encryption.

## License

MIT License
