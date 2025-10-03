package internal

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	PeerConn      *webrtc.PeerConnection
	VideoTrack    *webrtc.TrackLocalStaticRTP
	DataChannel   *webrtc.DataChannel
	Packetizer    rtp.Packetizer
	lastTimestamp uint32
	startTime     time.Time
	naluChan      chan []byte
	done          chan struct{}
	sentFrames    uint64
	droppedFrames uint64
}

type ClientManager struct {
	Clients      map[*Client]struct{}
	Mu           sync.RWMutex
	lastKeyframe []byte
	lastSPS      []byte
	lastPPS      []byte
}

type FrameStats struct {
	SentFrames    uint64 `json:"sentFrames"`
	DroppedFrames uint64 `json:"droppedFrames"`
	Timestamp     int64  `json:"timestamp"`
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients: make(map[*Client]struct{}),
	}
}

func (cm *ClientManager) BroadcastNALUs(naluChan <-chan []byte) {
	for nalu := range naluChan {
		cm.cacheKeyframes(nalu)

		cm.Mu.RLock()
		for c := range cm.Clients {
			select {
			case c.naluChan <- nalu:
			default:
				// Client can't keep up, skip frame
				c.droppedFrames++
			}
		}
		cm.Mu.RUnlock()
	}
}

func (cm *ClientManager) cacheKeyframes(nalu []byte) {
	if len(nalu) < 5 {
		return
	}
	naluType := nalu[4] & 0x1F
	switch naluType {
	case 7: // SPS
		cm.lastSPS = make([]byte, len(nalu))
		copy(cm.lastSPS, nalu)
	case 8: // PPS
		cm.lastPPS = make([]byte, len(nalu))
		copy(cm.lastPPS, nalu)
	case 5: // IDR
		cm.lastKeyframe = make([]byte, len(nalu))
		copy(cm.lastKeyframe, nalu)
	}
}

func (cm *ClientManager) AddClient(client *Client) {
	cm.Mu.Lock()
	cm.Clients[client] = struct{}{}
	cm.Mu.Unlock()

	// Send cached keyframes immediately
	if cm.lastSPS != nil {
		packets := client.Packetizer.Packetize(cm.lastSPS, 0)
		for _, pkt := range packets {
			_ = client.VideoTrack.WriteRTP(pkt)
		}
	}
	if cm.lastPPS != nil {
		packets := client.Packetizer.Packetize(cm.lastPPS, 0)
		for _, pkt := range packets {
			_ = client.VideoTrack.WriteRTP(pkt)
		}
	}
	if cm.lastKeyframe != nil {
		packets := client.Packetizer.Packetize(cm.lastKeyframe, 0)
		for _, pkt := range packets {
			_ = client.VideoTrack.WriteRTP(pkt)
		}
	}

	// Start per-client sender goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case nalu := <-client.naluChan:
				if client.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
					timestamp := uint32(time.Since(client.startTime).Milliseconds() * 90) // 90kHz clock
					packets := client.Packetizer.Packetize(nalu, timestamp-client.lastTimestamp)
					client.lastTimestamp = timestamp
					for _, pkt := range packets {
						_ = client.VideoTrack.WriteRTP(pkt)
					}
					client.sentFrames++
				}
			case <-ticker.C:
				// Send stats every second
				if client.DataChannel != nil && client.DataChannel.ReadyState() == webrtc.DataChannelStateOpen {
					stats := FrameStats{
						SentFrames:    client.sentFrames,
						DroppedFrames: client.droppedFrames,
						Timestamp:     time.Now().UnixMilli(),
					}
					data, err := json.Marshal(stats)
					if err == nil {
						_ = client.DataChannel.SendText(string(data))
					}
				}
			case <-client.done:
				return
			}
		}
	}()
}

func (cm *ClientManager) RemoveClient(client *Client) {
	cm.Mu.Lock()
	delete(cm.Clients, client)
	cm.Mu.Unlock()
	close(client.done)
}

func NewClient(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticRTP, dc *webrtc.DataChannel) *Client {
	ssrc := rand.Uint32()
	packetizer := rtp.NewPacketizer(
		1200, 96, ssrc, &codecs.H264Payloader{},
		rtp.NewRandomSequencer(), 90000,
	)
	naluChan := make(chan []byte, 100)
	done := make(chan struct{})
	return &Client{pc, track, dc, packetizer, 0, time.Now(), naluChan, done, 0, 0}
}
