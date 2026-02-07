#!/bin/bash
cd server
go mod tidy
cd ../client
npm ci
# Install Claude
curl -fsSL https://claude.ai/install.sh | bash