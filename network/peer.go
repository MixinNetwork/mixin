package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/vmihailenco/msgpack"
)

type Node interface {
	NetworkId() crypto.Hash
	BuildGraph() []SyncPoint
	FeedMempool(s *common.Snapshot) error
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
	ReadSnapshotByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error)
}

type Peer struct {
	IdForNetwork crypto.Hash
	Account      common.Address
	Address      string

	GraphChan chan []SyncPoint
	Neighbors map[crypto.Hash]*Peer
	Node      Node
	transport Transport
	send      chan []byte
}

type SyncPoint struct {
	NodeId crypto.Hash
	Number uint64
	Start  uint64
}

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

func (me *Peer) AddNeighbor(acc common.Address, addr string) {
	peer := NewPeer(nil, acc, addr)
	peerId := peer.Account.Hash()
	networkId := me.Node.NetworkId()
	peer.IdForNetwork = crypto.NewHash(append(networkId[:], peerId[:]...))

	if peer.Address == me.Address || me.Neighbors[peer.IdForNetwork] != nil {
		return
	}
	me.Neighbors[peer.IdForNetwork] = peer

	go func(p *Peer) {
		for {
			err := me.openPeerStream(p)
			if err != nil {
				logger.Println("neighbor open stream error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}(peer)

	go me.syncToNeighborLoop(peer)
}

func NewPeer(node Node, acc common.Address, addr string) *Peer {
	return &Peer{
		Node:      node,
		Account:   acc,
		Address:   addr,
		GraphChan: make(chan []SyncPoint),
		Neighbors: make(map[crypto.Hash]*Peer),
		send:      make(chan []byte, 8192),
	}
}

func (p *Peer) SendSnapshotMessage(s *common.Snapshot) error {
	return p.Send(buildSnapshotMessage(s))
}

func (p *Peer) Send(data []byte) error {
	select {
	case p.send <- data:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send timeout")
	}
}

func (me *Peer) ListenNeighbors() error {
	transport, err := NewQuicServer(me.Address, me.Account.PrivateSpendKey)
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

func buildAuthenticationMessage(id common.Address) []byte {
	data := make([]byte, 9)
	data[0] = PeerMessageTypeAuthentication
	binary.BigEndian.PutUint64(data[1:], uint64(time.Now().Unix()))
	hash := id.Hash()
	data = append(data, hash[:]...)
	sig := id.PrivateSpendKey.Sign(data[1:])
	return append(data, sig[:]...)
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

	err = client.Send(buildAuthenticationMessage(me.Account))
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
			err := client.Send(buildGraphMessage(me.Node.BuildGraph()))
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
			if msg.Snapshot.CheckSignature(peer.Account.PublicSpendKey) {
				me.Node.FeedMempool(msg.Snapshot)
			}
		case PeerMessageTypeGraph:
			peer.GraphChan <- msg.FinalCache
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
		ts := binary.BigEndian.Uint64(msg.Data[:8])
		if time.Now().Unix()-int64(ts) > 3 {
			auth <- errors.New("peer authentication message timeout")
			return
		}
		for _, p := range me.Neighbors {
			hash := p.Account.Hash()
			if !bytes.Equal(hash[:], msg.Data[8:40]) {
				continue
			}
			var sig crypto.Signature
			copy(sig[:], msg.Data[40:])
			if p.Account.PublicSpendKey.Verify(msg.Data[:40], sig) {
				peer = p
				auth <- nil
				return
			}
			break
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
