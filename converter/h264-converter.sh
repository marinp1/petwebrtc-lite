#!/bin/bash

# Usage: h264-converter.sh <watch_dir> [output_dir]
#   watch_dir:  directory to watch for .h264 files (required)
#   output_dir: directory for converted .mp4 files (defaults to watch_dir)

if [[ -z "$1" ]]; then
    echo "Usage: $0 <watch_dir> [output_dir]" >&2
    echo "  watch_dir:  directory to watch for .h264 files" >&2
    echo "  output_dir: directory for converted .mp4 files (defaults to watch_dir)" >&2
    exit 1
fi

WATCH_DIR="$1"
OUTPUT_DIR="${2:-$WATCH_DIR}"
LOG_FILE="/tmp/h264-converter.log"

if [[ ! -d "$WATCH_DIR" ]]; then
    echo "Error: watch_dir '$WATCH_DIR' does not exist" >&2
    exit 1
fi

if [[ ! -d "$OUTPUT_DIR" ]]; then
    echo "Error: output_dir '$OUTPUT_DIR' does not exist" >&2
    exit 1
fi

# Function to convert file
convert_file() {
    local input="$1"
    local filename=$(basename "$input")
    local dirname=$(dirname "$input")
    local basename="${filename%.h264}"

    # Extract camera name from directory
    local camera=$(basename "$dirname")

    # Get file modification time for datetime
    local datetime=$(stat -c %y "$input" | cut -d'.' -f1 | tr ' ' '_' | tr ':' '-')

    local output="$OUTPUT_DIR/${camera}_${datetime}.mp4"

    echo "[$(date)] Converting: $camera/$filename -> $(basename "$output")" >> "$LOG_FILE"

    # Your exact ffmpeg command - stderr to temp file
    local ffmpeg_log=$(mktemp)
    if ffmpeg -f h264 \
              -i "$input" \
              -c:v copy \
              -movflags +faststart \
              -y \
              "$output" 2>"$ffmpeg_log"; then
        echo "[$(date)] Completed: $(basename "$output")" >> "$LOG_FILE"
        rm "$ffmpeg_log"

        # Optional: delete original .h264 after successful conversion
        # rm "$input"
    else
        echo "[$(date)] ERROR: Failed to convert $camera/$filename" >> "$LOG_FILE"
        echo "ERROR: Failed to convert $camera/$filename" >&2
        cat "$ffmpeg_log" >&2
        rm "$ffmpeg_log"
        return 1
    fi
}

# Main watch loop - watch for moved_to events (file renames)
echo "[$(date)] Starting H264 converter, watching: $WATCH_DIR" >> "$LOG_FILE"
echo "H264 converter started, watching for .h264 files (renamed from .tmp)"

inotifywait -m -r -e moved_to --format '%w%f' "$WATCH_DIR" 2>&1 | while read file
do
    # Only process .h264 files in subdirectories that were just renamed
    if [[ "$file" == *.h264 ]]; then
        # Skip if file is directly in WATCH_DIR root
        if [[ "$(dirname "$file")" == "$WATCH_DIR" ]]; then
            continue
        fi

        # Verify the .tmp file doesn't exist anymore (confirming it was a rename)
        local tmp_file="${file%.h264}.tmp"
        if [[ ! -f "$tmp_file" ]]; then
            echo "[$(date)] New file detected (renamed from .tmp): $file" >> "$LOG_FILE"
            convert_file "$file" &
        fi
    fi
done
