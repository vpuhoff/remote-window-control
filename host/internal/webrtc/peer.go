package webrtc

import (
	"context"
	"sync"

	"share-app-host/internal/nativecapture"
	"share-app-host/internal/targetwindow"

	pion "github.com/pion/webrtc/v4"
)

type Peer struct {
	pc             *pion.PeerConnection
	onICECandidate func(pion.ICECandidateInit)
	onControl      func([]byte)
	mu             sync.Mutex
	cancel         context.CancelFunc
}

func NewPeer(bridge *nativecapture.Bridge, targets *targetwindow.Manager, onICE func(pion.ICECandidateInit), onControl func([]byte)) (*Peer, error) {
	api := pion.NewAPI()
	pc, err := api.NewPeerConnection(pion.Configuration{})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	peer := &Peer{
		pc:             pc,
		onICECandidate: onICE,
		onControl:      onControl,
		cancel:         cancel,
	}

	pc.OnICECandidate(func(candidate *pion.ICECandidate) {
		if candidate == nil || peer.onICECandidate == nil {
			return
		}
		peer.onICECandidate(candidate.ToJSON())
	})

	pc.OnDataChannel(func(channel *pion.DataChannel) {
		if channel.Label() != "control" {
			return
		}

		channel.OnMessage(func(message pion.DataChannelMessage) {
			if peer.onControl != nil {
				peer.onControl(message.Data)
			}
		})
	})

	if err := attachWindowVideoTrack(ctx, pc, bridge, targets); err != nil {
		return nil, err
	}

	return peer, nil
}

func (p *Peer) AcceptOffer(sdp string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.pc.SetRemoteDescription(pion.SessionDescription{
		Type: pion.SDPTypeOffer,
		SDP:  sdp,
	}); err != nil {
		return "", err
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}

	return answer.SDP, nil
}

func (p *Peer) AddICECandidate(candidate pion.ICECandidateInit) error {
	return p.pc.AddICECandidate(candidate)
}

func (p *Peer) Close() error {
	if p.cancel != nil {
		p.cancel()
	}
	return p.pc.Close()
}
