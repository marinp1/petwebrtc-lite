#!/bin/bash
set -euo pipefail
#set -x

APP_NAME="server-arm64"
REMOTE_HOST="ipcam"
REMOTE_DIR="~/opt/bin/ipcam"
REMOTE_PATH="$REMOTE_DIR/$APP_NAME"
LOCAL_BUILD="./builds/$APP_NAME"

# --- Deployment steps ---
echo "[1/3] Stopping remote process..."
ssh "$REMOTE_HOST" "pkill -f $APP_NAME" || true
echo "[1/3] Remote process stopped."

echo "[2/3] Deploying binary..."
scp "$LOCAL_BUILD" "$REMOTE_HOST:$REMOTE_PATH"

echo "[3/3] Starting remote process in background..."
ssh "$REMOTE_HOST" "nohup $REMOTE_PATH >/dev/null 2>&1 &"

echo "✅ Deployment complete."
