#!/bin/bash
set -euo pipefail
echo "Deploying client to caddy server..."
scp -r "./client/." "kontionkolo:~/opt/caddy/malva/"