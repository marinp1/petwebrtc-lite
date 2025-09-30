cd server
GOOS=linux GOARCH=arm64 go build -o ../builds/petwebrtc-arm64 .
GOOS=linux GOARCH=arm GOARM=7 go build -o ../builds/petwebrtc-arm32 .