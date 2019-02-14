package network

import (
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/vmihailenco/msgpack"
)

const (
	PeerMessageTypeSnapshot       = 0
	PeerMessageTypePing           = 1
	PeerMessageTypePong           = 2
	PeerMessageTypeAuthentication = 3
	PeerMessageTypeGraph          = 4
)

type PeerMessage struct {
	Type       uint8
	Snapshot   *common.Snapshot
	FinalCache []SyncPoint
	Data       []byte
}

type SyncHandle interface {
	BuildAuthenticationMessage() []byte
	Authenticate(msg []byte) (crypto.Hash, error)
	BuildGraph() []SyncPoint
	FeedMempool(peer *Peer, s *common.Snapshot) error
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
}

type SyncPoint struct {
	NodeId crypto.Hash
	Number uint64
}

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	neighbors map[crypto.Hash]*Peer
	handle    SyncHandle
	transport Transport
	send      chan []byte
	sync      chan []SyncPoint
}

func (me *Peer) AddNeighbor(idForNetwork crypto.Hash, addr string) {
	peer := NewPeer(nil, idForNetwork, addr)
	if peer.Address == me.Address || me.neighbors[peer.IdForNetwork] != nil {
		return
	}
	me.neighbors[peer.IdForNetwork] = peer

	go me.openPeerStreamLoop(peer)
	go me.syncToNeighborLoop(peer)
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string) *Peer {
	return &Peer{
		IdForNetwork: idForNetwork,
		Address:      addr,
		neighbors:    make(map[crypto.Hash]*Peer),
		send:         make(chan []byte, 8192),
		sync:         make(chan []SyncPoint),
		handle:       handle,
	}
}

func (me *Peer) SendSnapshotMessage(idForNetwork crypto.Hash, s *common.Snapshot) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	for _, p := range me.neighbors {
		if p.IdForNetwork == idForNetwork {
			return p.SendData(buildSnapshotMessage(s))
		}
	}
	return nil
}

func (p *Peer) SendData(data []byte) error {
	select {
	case p.send <- data:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send timeout")
	}
}

func (me *Peer) ListenNeighbors() error {
	transport, err := NewQuicServer(me.Address)
	if err != nil {
		return err
	}
	me.transport = transport

	err = me.transport.Listen()
	if err != nil {
		return err
	}

	for {
		c, err := me.transport.Accept()
		if err != nil {
			return err
		}
		go me.acceptNeighborConnection(c)
	}
}

func parseNetworkMessage(data []byte) (*PeerMessage, error) {
	if len(data) < 1 {
		return nil, errors.New("invalid message data")
	}
	msg := &PeerMessage{Type: data[0]}
	switch msg.Type {
	case PeerMessageTypeSnapshot:
		var ss common.Snapshot
		err := msgpack.Unmarshal(data[1:], &ss)
		if err != nil {
			return nil, err
		}
		msg.Snapshot = &ss
	case PeerMessageTypeGraph:
		err := msgpack.Unmarshal(data[1:], &msg.FinalCache)
		if err != nil {
			return nil, err
		}
	case PeerMessageTypePing, PeerMessageTypePong:
	case PeerMessageTypeAuthentication:
		msg.Data = data[1:]
	}
	return msg, nil
}

func buildAuthenticationMessage(data []byte) []byte {
	header := []byte{PeerMessageTypeAuthentication}
	return append(header, data...)
}

func buildPingMessage() []byte {
	return []byte{PeerMessageTypePing}
}

func buildPongMessage() []byte {
	return []byte{PeerMessageTypePong}
}

func buildSnapshotMessage(ss *common.Snapshot) []byte {
	data := common.MsgpackMarshalPanic(ss)
	return append([]byte{PeerMessageTypeSnapshot}, data...)
}

func buildGraphMessage(points []SyncPoint) []byte {
	data := common.MsgpackMarshalPanic(points)
	return append([]byte{PeerMessageTypeGraph}, data...)
}

func (me *Peer) openPeerStreamLoop(p *Peer) {
	for {
		err := me.openPeerStream(p)
		if err != nil {
			logger.Println("neighbor open stream error", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func (me *Peer) openPeerStream(peer *Peer) error {
	logger.Println("OPEN PEER STREAM", peer.Address)
	transport, err := NewQuicClient(peer.Address)
	if err != nil {
		return err
	}
	client, err := transport.Dial()
	if err != nil {
		return err
	}
	defer client.Close()
	logger.Println("DIAL PEER STREAM", peer.Address)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return err
	}
	logger.Println("AUTH PEER STREAM", peer.Address)

	go func() error {
		defer client.Close()

		for {
			data, err := client.Receive()
			if err != nil {
				return err
			}
			_, err = parseNetworkMessage(data)
			if err != nil {
				return err
			}
		}
	}()

	pingTicker := time.NewTicker(1 * time.Second)
	defer pingTicker.Stop()

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap / 2))
	defer graphTicker.Stop()

	logger.Println("LOOP PEER STREAM", peer.Address)
	for {
		select {
		case msg := <-peer.send:
			err := client.Send(msg)
			if err != nil {
				return err
			}
		case <-graphTicker.C:
			err := client.Send(buildGraphMessage(me.handle.BuildGraph()))
			if err != nil {
				return err
			}
		case <-pingTicker.C:
			err := client.Send(buildPingMessage())
			if err != nil {
				return err
			}
		}
	}
}

func (me *Peer) acceptNeighborConnection(client Client) error {
	defer client.Close()

	peer, err := me.authenticateNeighbor(client)
	if err != nil {
		logger.Println("peer authentication error", err)
		return err
	}

	for {
		data, err := client.Receive()
		if err != nil {
			return err
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			return err
		}
		switch msg.Type {
		case PeerMessageTypePing:
			err = client.Send(buildPongMessage())
			if err != nil {
				return err
			}
		case PeerMessageTypeSnapshot:
			me.handle.FeedMempool(peer, msg.Snapshot)
		case PeerMessageTypeGraph:
			peer.sync <- msg.FinalCache
		}
	}
}

func (me *Peer) authenticateNeighbor(client Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		data, err := client.Receive()
		if err != nil {
			auth <- err
			return
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			auth <- err
			return
		}
		if msg.Type != PeerMessageTypeAuthentication {
			auth <- errors.New("peer authentication invalid message type")
			return
		}

		id, err := me.handle.Authenticate(msg.Data)
		if err != nil {
			auth <- err
			return
		}
		for _, p := range me.neighbors {
			if id != p.IdForNetwork {
				continue
			}
			peer = p
			auth <- nil
			return
		}
		auth <- errors.New("peer authentication message signature invalid")
	}()

	select {
	case err := <-auth:
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("peer authentication failed %s", err.Error())
		}
	case <-time.After(3 * time.Second):
		client.Close()
		return nil, errors.New("peer authentication timeout")
	}
	return peer, nil
}
