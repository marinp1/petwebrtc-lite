# Deployment Guide

This guide covers deploying PetWebRTC to Raspberry Pi devices, including optional NAS storage for recordings.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Architecture Overview](#architecture-overview)
- [1. Camera Server Deployment](#1-camera-server-deployment)
- [2. NAS Storage Setup (Optional)](#2-nas-storage-setup-optional)
- [3. H264 Converter Service](#3-h264-converter-service)
- [4. Web Access to Recordings](#4-web-access-to-recordings)
- [Service Management](#service-management)

## Prerequisites

**On Camera Pis (Zero 2W, Pi 4, etc.):**
- Raspberry Pi OS
- [rpicam-vid](https://www.raspberrypi.com/documentation/computers/camera_software.html) for camera access

**On NAS Server (Pi 5 or similar):**
- `ffmpeg` for video conversion
- `inotify-tools` for file watching (`sudo apt install inotify-tools`)
- `nfs-kernel-server` for network storage

## Architecture Overview

```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│  Camera Pi 1    │         │  Camera Pi 2    │         │  NAS Server     │
│  (Zero 2W)      │         │  (Pi 4)         │         │  (Pi 5)         │
│                 │         │                 │         │                 │
│  petwebrtc      │ ──────▶ │  petwebrtc      │ ──────▶ │  NFS Server     │
│  service        │  WiFi   │  service        │ Network │  H264 Converter │
│                 │         │                 │         │  Web Server     │
└─────────────────┘         └─────────────────┘         └─────────────────┘
         │                           │                           │
         └───────────────────────────┴───────────────────────────┘
                       All write to: /mnt/nas/recordings/
```

## 1. Camera Server Deployment

### 1.1 Deploy the Server Binary

Build and deploy from your development machine:

```bash
# Build for ARM
./scripts/build.sh

# Deploy to camera Pi
./scripts/deploy-server.sh deploy-start <hostname>
```

### 1.2 Create the Systemd Service

On each Camera Pi, create `/etc/systemd/system/petwebrtc.service`:

```ini
[Unit]
Description=PetWebRTC Server
After=network.target

[Service]
Type=simple
User=<username>
Group=<username>
WorkingDirectory=/home/<username>/opt/bin/ipcam
ExecStart=/home/<username>/opt/bin/ipcam/server-arm64 config/server.conf
Restart=on-failure
RestartSec=10

# Optional: discard logs (or use journalctl)
StandardOutput=null
StandardError=null

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable petwebrtc
sudo systemctl start petwebrtc
```

### 1.3 Configure the Server

Edit `config/server.conf` on each camera:

```ini
addr = 8765
width = 1280
height = 720
framerate = 30
rotation = 180

# For recording to NAS (see section 2)
recording_dir = /mnt/nas
```

---

## 2. NAS Storage Setup (Optional)

Use NFS to offload recording storage and video conversion from camera devices.

### 2.1 NFS Server Setup (on Pi 5 / NAS)

**Install NFS server:**
```bash
sudo apt update
sudo apt install nfs-kernel-server -y
```

**Create storage directories:**
```bash
sudo mkdir -p /srv/nfs/camera-recordings
sudo chown nobody:nogroup /srv/nfs/camera-recordings
sudo chmod 777 /srv/nfs/camera-recordings

# Create subdirectory for each camera
sudo mkdir -p /srv/nfs/camera-recordings/front-door
sudo mkdir -p /srv/nfs/camera-recordings/backyard
```

**Configure exports** - edit `/etc/exports`:
```
# Allow specific camera IPs (recommended)
/srv/nfs/camera-recordings 192.168.1.100(rw,sync,no_subtree_check,no_root_squash)
/srv/nfs/camera-recordings 192.168.1.101(rw,sync,no_subtree_check,no_root_squash)

# OR allow entire subnet
/srv/nfs/camera-recordings 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)
```

**Apply and start:**
```bash
sudo exportfs -ra
sudo systemctl enable nfs-kernel-server
sudo systemctl restart nfs-kernel-server

# Verify
showmount -e localhost
```

**Firewall (if enabled):**
```bash
sudo ufw allow from 192.168.1.0/24 to any port nfs
sudo ufw reload
```

### 2.2 NFS Client Setup (on Camera Pis)

**Install NFS client:**
```bash
sudo apt update
sudo apt install nfs-common -y
```

**Create mount point and test:**
```bash
sudo mkdir -p /mnt/nas
sudo mount -t nfs pi5.local:/srv/nfs/camera-recordings/front-door /mnt/nas
df -h | grep nas
```

**Make permanent** - add to `/etc/fstab`:
```
pi5.local:/srv/nfs/camera-recordings/front-door /mnt/nas nfs defaults,_netdev,noatime,nofail 0 0
```

Options:
- `_netdev` - Wait for network before mounting
- `noatime` - Don't update access times (performance)
- `nofail` - Don't fail boot if mount unavailable

**Test auto-mount:**
```bash
sudo umount /mnt/nas
sudo mount -a
ls -la /mnt/nas
```

### 2.3 Configure Recording

On each Camera Pi, update `config/server.conf`:
```ini
recording_dir = /mnt/nas
```

To skip local FFmpeg conversion (let NAS handle it):
```bash
export SKIP_FFMPEG_CONVERSION=1
```

Or add to the systemd service `[Service]` section:
```ini
Environment=SKIP_FFMPEG_CONVERSION=1
```

---

## 3. H264 Converter Service

Run on the NAS server to automatically convert `.h264` recordings to `.mp4`.

### 3.1 Install Prerequisites

```bash
sudo apt install ffmpeg inotify-tools -y
```

### 3.2 Deploy the Converter

Copy the `converter/` folder to your NAS server, then:

```bash
cd converter
chmod +x h264-converter.sh register.sh

# Register as systemd service
./register.sh /srv/nfs/camera-recordings
```

With separate output directory:
```bash
./register.sh /srv/nfs/camera-recordings /srv/converted
```

The converter watches for `.h264` files renamed from `.tmp` (indicating recording complete) and converts them to MP4.

---

## 4. Web Access to Recordings

Optionally serve recordings via HTTP for easy download.

### Using nginx

```bash
sudo apt install nginx -y
```

Create `/etc/nginx/sites-available/recordings`:
```nginx
server {
    listen 8080;
    server_name _;

    location / {
        root /srv/nfs/camera-recordings;
        autoindex on;
        autoindex_exact_size off;
        autoindex_localtime on;
    }
}
```

Enable and start:
```bash
sudo ln -s /etc/nginx/sites-available/recordings /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

Access at `http://<nas-ip>:8080/`

---

## Debugging

### Enable/Disable Logging

By default, server logs are discarded. To view them:

```bash
# Live tail logs
sudo journalctl -u petwebrtc -f

# Recent logs (last 5 minutes)
sudo journalctl -u petwebrtc --since "5 minutes ago"
```

To enable logs, create a service override:

```bash
sudo systemctl edit petwebrtc
```

Add:
```ini
[Service]
StandardOutput=journal
StandardError=journal
```

To remove logging, remove the override:

```bash
sudo systemctl revert petwebrtc
sudo systemctl restart petwebrtc
```

## Service Management

### PetWebRTC (Camera Pis)

```bash
sudo systemctl status petwebrtc
sudo systemctl start petwebrtc
sudo systemctl stop petwebrtc
sudo systemctl restart petwebrtc
sudo journalctl -u petwebrtc -f
```

### H264 Converter (NAS)

```bash
sudo systemctl status petwebrtc-h264-converter
sudo systemctl start petwebrtc-h264-converter
sudo systemctl stop petwebrtc-h264-converter
journalctl -u petwebrtc-h264-converter -f
```

### NFS Server (NAS)

```bash
sudo systemctl status nfs-kernel-server
sudo exportfs -v          # List active exports
showmount -e localhost    # Show exported directories
```
