#!/bin/bash
set -euo pipefail
#set -x

APP_NAME="petwebrtc-arm64"
REMOTE_HOST="ipcam"
REMOTE_DIR="~/opt/bin/ipcam"
REMOTE_PATH="$REMOTE_DIR/$APP_NAME"
LOCAL_BUILD="./builds/$APP_NAME"

# --- Check parameter ---
if [ $# -ne 1 ]; then
    echo "Usage: $0 deploy|deploy-start"
    exit 1
fi

MODE="$1"
if [[ "$MODE" != "deploy" && "$MODE" != "deploy-start" ]]; then
    echo "Invalid mode: $MODE"
    echo "Usage: $0 deploy|deploy-start"
    exit 1
fi

# --- Always stop remote process ---
echo "[1/3] Stopping remote process..."
ssh "$REMOTE_HOST" "pkill -f $APP_NAME" || true
echo "[1/3] Remote process stopped."

# --- Deploy binary ---
echo "[2/3] Deploying binary..."
scp "$LOCAL_BUILD" "$REMOTE_HOST:$REMOTE_PATH"
echo "[2/3] Deployment complete."

# --- Start remote process only if deploy-start ---
if [ "$MODE" == "deploy-start" ]; then
    echo "[3/3] Starting remote process in background..."
    ssh "$REMOTE_HOST" "nohup $REMOTE_PATH >/dev/null 2>&1 &"
    echo "[3/3] Remote process started."
else
    echo "[3/3] Skipped starting remote process."
fi

echo "✅ Deployment complete."
