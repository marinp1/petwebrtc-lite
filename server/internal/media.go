package internal

import (
	"math/rand"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	PeerConn   *webrtc.PeerConnection
	VideoTrack *webrtc.TrackLocalStaticRTP
	Packetizer rtp.Packetizer
}

type ClientManager struct {
	Clients map[*Client]struct{}
	Mu      sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients: make(map[*Client]struct{}),
	}
}

func (cm *ClientManager) BroadcastNALUs(naluChan <-chan []byte) {
	for nalu := range naluChan {
		cm.Mu.RLock()
		clients := make([]*Client, 0, len(cm.Clients))
		for c := range cm.Clients {
			clients = append(clients, c)
		}
		cm.Mu.RUnlock()

		for _, c := range clients {
			if c.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
				packets := c.Packetizer.Packetize(nalu, 90000/30)
				for _, pkt := range packets {
					_ = c.VideoTrack.WriteRTP(pkt)
				}
			}
		}
	}
}

func (cm *ClientManager) AddClient(client *Client) {
	cm.Mu.Lock()
	cm.Clients[client] = struct{}{}
	cm.Mu.Unlock()
}

func (cm *ClientManager) RemoveClient(client *Client) {
	cm.Mu.Lock()
	delete(cm.Clients, client)
	cm.Mu.Unlock()
}

func NewClient(pc *webrtc.PeerConnection, track *webrtc.TrackLocalStaticRTP) *Client {
	ssrc := rand.Uint32()
	packetizer := rtp.NewPacketizer(
		1200, 96, ssrc, &codecs.H264Payloader{},
		rtp.NewRandomSequencer(), 90000,
	)
	return &Client{pc, track, packetizer}
}
