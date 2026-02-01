
# PetWebRTC-Lite

PetWebRTC-Lite is a lightweight WebRTC streaming server for live H264 video from Raspberry Pi cameras to web browsers. The server is written in Go using [Pion WebRTC](https://github.com/pion/webrtc), and the client is a modern TypeScript/Vite application with optional MediaPipe object detection.

## Features

- **Low-latency WebRTC streaming** from Raspberry Pi camera to any modern web browser
- **Multi-camera support** with carousel UI and swipe navigation
- **H264 recording** with on-demand start/stop and downloadable recordings
- **MediaPipe object detection** with canvas overlay (optional)
- **Mobile-first responsive design** with touch gestures and keyboard navigation
- **Connection health monitoring** with automatic reconnect and status indicators
- **Minimal dependencies** - runs efficiently on Raspberry Pi Zero 2 W and up
- **Cross-compilation** support for ARM64 and ARM32 architectures

## Architecture

### Components

- **Server** ([server/](server/)): Go application that manages camera processes, parses H264 NAL units, handles WebRTC signaling, and streams video to clients
- **Client** ([client/](client/)): TypeScript/Vite web application with carousel UI, MediaPipe detection, and recording controls
- **Config** ([server/config/server.conf](server/config/server.conf)): Configuration file for camera and server settings

### Data Flow

1. **Camera → Server**: `rpicam-vid` outputs H264 byte stream → server parses NAL units (SPS, PPS, IDR, P-frames) → broadcasts to clients and optional recorder
2. **Server → Clients**: RTP H264 packetization (MTU=1200) → WebRTC peer connections
3. **Client Rendering**: Browser receives video track → displays in `<video>` element → optional MediaPipe object detection on canvas overlay

## Getting Started

### Prerequisites

- Go 1.23+ (for building the server)
- Node.js 18+ (for building the client)
- Raspberry Pi OS or Linux (for camera streaming)
- [rpicam-vid](https://www.raspberrypi.com/documentation/computers/camera_software.html) (Raspberry Pi camera support)

### Build the Server

```bash
# Build for local architecture
cd server
go build -o ../builds/server-local .

# Cross-compile for Raspberry Pi (ARM64 and ARM32)
./scripts/build.sh
```

Binaries are output to `builds/server-arm64` and `builds/server-arm32`.

### Build the Client

```bash
cd client
npm install
npm run build
```

Production build output goes to `client/dist/`.

### Deploy to Raspberry Pi

Edit the deployment scripts to set your `REMOTE_HOST` and `REMOTE_DIR`:

```bash
# Deploy server binary only
./scripts/deploy-server.sh deploy ipcam

# Deploy server binary and start it
./scripts/deploy-server.sh deploy-start ipcam

# Deploy client assets
./scripts/deploy-client.sh
```

### Set up as a System Service

Create service file `/etc/systemd/system/petwebrtc.service`:

```ini
[Unit]
Description=PetWebRTC Server
After=network.target

[Service]
ExecStart=/home/<username>/opt/bin/ipcam/server-arm64
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

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable petwebrtc.service
sudo systemctl start petwebrtc.service
sudo systemctl status petwebrtc.service
```

## Configuration

Edit [server/config/server.conf](server/config/server.conf):

```ini
addr = 8765               # Server port
width = 1280              # Camera resolution
height = 720
framerate = 30
rotation = 180            # Camera rotation (0, 90, 180, 270)
recording_dir = /path/to/recordings  # Optional: enable recording feature
```

## API Endpoints

### WebRTC Signaling
- `POST /offer` - Accepts WebRTC SDP offer, returns SDP answer
- `GET /cameras` - Returns camera count for multi-camera setups
- `GET /status` - Server health check

### Recording (when `recording_dir` is configured)
- `GET /record/status` - Get current recording status (idle/recording) and duration
- `POST /record/start` - Start H264 recording
- `POST /record/stop` - Stop recording and save file
- `GET /record/list` - List all recordings with metadata (date, duration, size)
- `GET /record/download/{filename}` - Download a recording file

## Development

### Client Development

```bash
cd client

# Start dev server with backend proxy
VITE_PROXY_TARGET=http://localhost:8765 npm run dev

# Format code with Biome
npm run format

# Preview production build
npm run preview
```

Dev server runs at `http://localhost:5173`.

### Server Development

```bash
cd server

# Run directly
go run . config/server.conf

# Build for local testing
go build -o ../builds/server-local .
```

## Client Features

### Carousel UI
- **Swipe navigation** - Touch gestures with velocity detection and edge bounce
- **Keyboard shortcuts** - Arrow keys, number keys (1-9), Home/End
- **Lazy loading** - Connects only to current and adjacent cameras for efficiency
- **Persistent state** - Remembers last viewed camera

### Connection Monitoring
- **Visual status indicators** - Color-coded banners for connecting/reconnecting/disconnected states
- **Health detection** - Automatic degraded/poor connection warnings based on dropped frames
- **Manual reconnect** - Button to recover from failed connections without page refresh

### Object Detection (Optional)
- **MediaPipe Vision** - Real-time object detection overlay
- **DPI-aware canvas** - High-resolution detection visualization
- **Catmull-Rom splines** - Smooth bounding box rendering
- **Persistent toggle** - Detection state saved to localStorage

### Recording Controls
- **Record button** - Start/stop recording with pulsing indicator
- **Recordings panel** - View all recordings with date, duration, size
- **Download** - Direct download links for saved recordings
- **Live duration** - Real-time recording timer

## Project Structure

```
.
├── server/                 # Go server source
│   ├── main.go            # HTTP server, signaling endpoint
│   ├── internal/          # Core server logic
│   │   ├── camera.go      # Camera process management, H264 parsing
│   │   ├── media.go       # Client manager, RTP packetization
│   │   ├── signaling.go   # WebRTC offer/answer exchange
│   │   ├── recorder.go    # H264 recording to disk
│   │   └── recording_handlers.go  # Recording HTTP endpoints
│   └── config/            # Configuration files
├── client/                # TypeScript/Vite web client
│   ├── src/
│   │   ├── main.ts        # Entry point, carousel setup
│   │   ├── carousel.ts    # Carousel controller
│   │   ├── connect.ts     # WebRTC connection management
│   │   ├── detector.ts    # MediaPipe object detection
│   │   ├── recording.ts   # Recording controls
│   │   ├── recordings-panel.ts  # Recordings list UI
│   │   ├── gestures.ts    # Touch/swipe handling
│   │   ├── navigation.ts  # Dots and arrow navigation
│   │   └── styles/        # Modular CSS files
│   ├── vite.config.ts     # Vite configuration
│   └── index.html         # HTML entry point
├── builds/                # Compiled server binaries
└── scripts/               # Build and deployment scripts
```

## Dependencies

### Server (Go)
- [Pion WebRTC](https://github.com/pion/webrtc) v4.1.4
- [Pion RTP](https://github.com/pion/rtp) v1.8.21
- Go 1.23.11

### Client (TypeScript)
- [MediaPipe Vision](https://www.npmjs.com/package/@mediapipe/tasks-vision) v0.10.22-rc
- TypeScript 5.8.3
- Vite (Rolldown bundler)
- Biome 2.2.4 (formatter/linter)

## Performance Optimizations

- **Large buffered reader** (256KB) to minimize syscalls when reading H264 stream
- **Non-blocking channel sends** - Drops frames when buffer fills instead of blocking camera
- **Keyframe caching** - New clients receive cached SPS/PPS/IDR frames for immediate playback
- **Lazy connection loading** - Only maintains WebRTC connections to visible cameras
- **Buffered writes** (64KB) for recording to reduce I/O overhead on Pi Zero 2 W

## Security

This server has no built-in authentication. Use behind a reverse proxy (nginx, Caddy) on untrusted networks. WebRTC uses DTLS-SRTP for media encryption, but HTTP endpoints are unprotected.

## License

MIT License
