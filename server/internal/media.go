package internal

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	PeerConn      *webrtc.PeerConnection
	VideoTrack    *webrtc.TrackLocalStaticRTP
	DataChannel   *webrtc.DataChannel
	dcMu          sync.RWMutex // Protects DataChannel access
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
				atomic.AddUint64(&c.droppedFrames, 1)
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
			case nalu, ok := <-client.naluChan:
				if !ok {
					return
				}
				if client.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
					timestamp := uint32(time.Since(client.startTime).Milliseconds() * 90) // 90kHz clock
					packets := client.Packetizer.Packetize(nalu, timestamp-client.lastTimestamp)
					client.lastTimestamp = timestamp
					for _, pkt := range packets {
						_ = client.VideoTrack.WriteRTP(pkt)
					}
					atomic.AddUint64(&client.sentFrames, 1)
				}
			case <-ticker.C:
				// Send stats every second
				client.dcMu.RLock()
				dc := client.DataChannel
				client.dcMu.RUnlock()

				if dc != nil && dc.ReadyState() == webrtc.DataChannelStateOpen {
					sent := atomic.LoadUint64(&client.sentFrames)
					dropped := atomic.LoadUint64(&client.droppedFrames)

					stats := FrameStats{
						SentFrames:    sent,
						DroppedFrames: dropped,
						Timestamp:     time.Now().UnixMilli(),
					}
					data, err := json.Marshal(stats)
					if err == nil {
						if err := dc.SendText(string(data)); err != nil {
							log.Printf("Error sending stats: %v", err)
						}
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

	// Close done channel to stop goroutine
	select {
	case <-client.done:
		// Already closed
	default:
		close(client.done)
	}
}

// SetDataChannel safely sets the data channel for a client
func (c *Client) SetDataChannel(dc *webrtc.DataChannel) {
	c.dcMu.Lock()
	c.DataChannel = dc
	c.dcMu.Unlock()
}

func NewClient(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticRTP, dc *webrtc.DataChannel) *Client {
	ssrc := rand.Uint32()
	packetizer := rtp.NewPacketizer(
		1200, 96, ssrc, &codecs.H264Payloader{},
		rtp.NewRandomSequencer(), 90000,
	)
	naluChan := make(chan []byte, 100)
	done := make(chan struct{})
	return &Client{
		PeerConn:    pc,
		VideoTrack:  track,
		DataChannel: dc,
		Packetizer:  packetizer,
		startTime:   time.Now(),
		naluChan:    naluChan,
		done:        done,
	}
}
