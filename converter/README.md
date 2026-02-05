# H264 to MP4 Converter Service

Watches for `.h264` recording files and converts them to MP4 using ffmpeg. Runs on your NAS server to offload conversion from camera devices.

For full deployment instructions including NAS/NFS setup, see [docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md).

## Prerequisites

- `ffmpeg` installed
- `inotify-tools` installed (`sudo apt install inotify-tools`)

## Installation

1. Make the scripts executable:
   ```bash
   chmod +x h264-converter.sh register.sh
   ```

2. Register and start the service:
   ```bash
   ./register.sh /path/to/watch/dir
   ```

   With separate output directory:
   ```bash
   ./register.sh /path/to/watch/dir /path/to/output/dir
   ```

   Specify a different user:
   ```bash
   ./register.sh /path/to/watch/dir /path/to/output/dir pi
   ```

## Manual Usage

Run the converter directly without installing as a service:
```bash
./h264-converter.sh /path/to/watch/dir [/path/to/output/dir]
```

## Service Management

```bash
# Check status
sudo systemctl status petwebrtc-h264-converter

# View logs
journalctl -u petwebrtc-h264-converter -f

# Stop/start/restart
sudo systemctl stop petwebrtc-h264-converter
sudo systemctl start petwebrtc-h264-converter
sudo systemctl restart petwebrtc-h264-converter

# Disable auto-start on boot
sudo systemctl disable petwebrtc-h264-converter
```