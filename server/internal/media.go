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
	tsInc         uint32 // timestamp increment per frame (90kHz / fps)
	startTime     time.Time
	naluChan      chan []byte
	done          chan struct{}
	wg            sync.WaitGroup // Tracks sender goroutine
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

const maxPayloadSize = 1200 // MTU for packetizer

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

	// Send cached keyframes immediately (use MTU and set proper timestamp)
	// advance timestamp for each logical frame sent to keep monotonic RTP timestamps
	if client.tsInc == 0 {
		// fallback to 30fps if not set
		client.tsInc = 90000 / 30
	}
	if cm.lastSPS != nil {
		client.lastTimestamp += client.tsInc
		packets := client.Packetizer.Packetize(cm.lastSPS, maxPayloadSize)
		for _, pkt := range packets {
			pkt.Header.Timestamp = client.lastTimestamp
			if err := client.VideoTrack.WriteRTP(pkt); err != nil {
				log.Printf("WriteRTP error (SPS): %v", err)
			}
		}
	}
	if cm.lastPPS != nil {
		client.lastTimestamp += client.tsInc
		packets := client.Packetizer.Packetize(cm.lastPPS, maxPayloadSize)
		for _, pkt := range packets {
			pkt.Header.Timestamp = client.lastTimestamp
			if err := client.VideoTrack.WriteRTP(pkt); err != nil {
				log.Printf("WriteRTP error (PPS): %v", err)
			}
		}
	}
	if cm.lastKeyframe != nil {
		client.lastTimestamp += client.tsInc
		packets := client.Packetizer.Packetize(cm.lastKeyframe, maxPayloadSize)
		for _, pkt := range packets {
			pkt.Header.Timestamp = client.lastTimestamp
			if err := client.VideoTrack.WriteRTP(pkt); err != nil {
				log.Printf("WriteRTP error (Keyframe): %v", err)
			}
		}
	}

	// Start per-client sender goroutine
	client.wg.Add(1)
	go func() {
		defer client.wg.Done()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case nalu, ok := <-client.naluChan:
				if !ok {
					return
				}
				if client.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
					// Advance timestamp once per frame for monotonic timestamps
					if client.tsInc == 0 {
						client.tsInc = 90000 / 30 // fallback
					}
					client.lastTimestamp += client.tsInc
					timestamp := client.lastTimestamp

					packets := client.Packetizer.Packetize(nalu, maxPayloadSize)
					for _, pkt := range packets {
						pkt.Header.Timestamp = timestamp
						if err := client.VideoTrack.WriteRTP(pkt); err != nil {
							// Log but don't block; underlying connection state will handle cleanup
							log.Printf("WriteRTP error: %v", err)
						}
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

	// Wait for sender goroutine to finish
	client.wg.Wait()

	// Close NALU channel
	close(client.naluChan)
}

// SetDataChannel safely sets the data channel for a client
func (c *Client) SetDataChannel(dc *webrtc.DataChannel) {
	c.dcMu.Lock()
	c.DataChannel = dc
	c.dcMu.Unlock()
}

func NewClient(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticRTP, dc *webrtc.DataChannel, fps int) *Client {
	ssrc := rand.Uint32()
	packetizer := rtp.NewPacketizer(
		maxPayloadSize, 96, ssrc, &codecs.H264Payloader{},
		rtp.NewRandomSequencer(), 90000,
	)
	// Increase per-client NALU buffer to tolerate bursts
	naluChan := make(chan []byte, 500)
	done := make(chan struct{})

	if fps <= 0 {
		fps = 30
	}
	tsInc := uint32(90000 / fps)

	return &Client{
		PeerConn:      pc,
		VideoTrack:    track,
		DataChannel:   dc,
		Packetizer:    packetizer,
		lastTimestamp: 0,
		tsInc:         tsInc,
		startTime:     time.Now(),
		naluChan:      naluChan,
		done:          done,
	}
}
