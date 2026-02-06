#!/bin/bash

# Register the H264 converter as a systemd service
# Usage: ./register.sh <watch_dir> [output_dir] [username]

set -e

if [[ -z "$1" ]]; then
    echo "Usage: $0 <watch_dir> [output_dir] [username]" >&2
    echo "  watch_dir:  directory to watch for .h264 files (required)" >&2
    echo "  output_dir: directory for converted .mp4 files (defaults to watch_dir)" >&2
    echo "  username:   user to run the service as (defaults to current user)" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_NAME="petwebrtc-h264-converter"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

WATCH_DIR="$1"
OUTPUT_DIR="${2:-$WATCH_DIR}"
USERNAME="${3:-$(whoami)}"

if [[ ! -d "$WATCH_DIR" ]]; then
    echo "Error: watch_dir '$WATCH_DIR' does not exist" >&2
    exit 1
fi

if [[ ! -d "$OUTPUT_DIR" ]]; then
    echo "Error: output_dir '$OUTPUT_DIR' does not exist" >&2
    exit 1
fi

echo "Registering ${SERVICE_NAME} service..."
echo "  User: ${USERNAME}"
echo "  Watch dir: ${WATCH_DIR}"
echo "  Output dir: ${OUTPUT_DIR}"
echo "  Script path: ${SCRIPT_DIR}/h264-converter.sh"

# Make sure the converter script is executable
chmod +x "${SCRIPT_DIR}/h264-converter.sh"

# Read template and substitute variables
SERVICE_CONTENT=$(cat "${SCRIPT_DIR}/h264-service.ini" \
    | sed "s|\$\$USERNAME|${USERNAME}|g" \
    | sed "s|\$\$PATH_TO_FOLDER|${SCRIPT_DIR}|g" \
    | sed "s|\$\$WATCH_DIR|${WATCH_DIR}|g" \
    | sed "s|\$\$OUTPUT_DIR|${OUTPUT_DIR}|g")

# Write the service file
echo "Writing service file to ${SERVICE_FILE}..."
echo "${SERVICE_CONTENT}" | sudo tee "${SERVICE_FILE}" > /dev/null

# Reload systemd and enable/start the service
echo "Reloading systemd daemon..."
sudo systemctl daemon-reload

echo "Enabling ${SERVICE_NAME}..."
sudo systemctl enable "${SERVICE_NAME}"

echo "Starting ${SERVICE_NAME}..."
sudo systemctl start "${SERVICE_NAME}"

echo ""
echo "Service registered and started successfully!"
echo "  Check status: sudo systemctl status ${SERVICE_NAME}"
echo "  View logs:    journalctl -u ${SERVICE_NAME} -f"
