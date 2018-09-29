package kernel

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
	"github.com/MixinNetwork/mixin/network"
	"github.com/vmihailenco/msgpack"
)

type Peer struct {
	IdForNetwork crypto.Hash
	Account      common.Address
	Address      string

	GraphChan chan []FinalRound
	send      chan []byte
}

const (
	MessageTypeSnapshot       = 0
	MessageTypePing           = 1
	MessageTypePong           = 2
	MessageTypeAuthentication = 3
	MessageTypeGraph          = 4
)

type Message struct {
	Type       uint8
	Snapshot   *common.Snapshot
	FinalCache []FinalRound
	Data       []byte
}

func NewPeer(acc common.Address, addr string) *Peer {
	return &Peer{
		Account:   acc,
		Address:   addr,
		GraphChan: make(chan []FinalRound),
		send:      make(chan []byte, 8192),
	}
}

func (p *Peer) Send(data []byte) error {
	select {
	case p.send <- data:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send timeout")
	}
}

func (node *Node) ListenPeers() error {
	err := node.transport.Listen()
	if err != nil {
		return err
	}

	for {
		c, err := node.transport.Accept()
		if err != nil {
			return err
		}
		go node.acceptPeerStream(c)
	}
}

func parseNetworkMessage(data []byte) (*Message, error) {
	if len(data) < 1 {
		return nil, errors.New("invalid message data")
	}
	msg := &Message{Type: data[0]}
	switch msg.Type {
	case MessageTypeSnapshot:
		var ss common.Snapshot
		err := msgpack.Unmarshal(data[1:], &ss)
		if err != nil {
			return nil, err
		}
		msg.Snapshot = &ss
	case MessageTypeGraph:
		err := msgpack.Unmarshal(data[1:], &msg.FinalCache)
		if err != nil {
			return nil, err
		}
	case MessageTypePing, MessageTypePong:
	case MessageTypeAuthentication:
		msg.Data = data[1:]
	}
	return msg, nil
}

func buildAuthenticationMessage(id common.Address) []byte {
	data := make([]byte, 9)
	data[0] = MessageTypeAuthentication
	binary.BigEndian.PutUint64(data[1:], uint64(time.Now().Unix()))
	hash := id.Hash()
	data = append(data, hash[:]...)
	sig := id.PrivateSpendKey.Sign(data[1:])
	return append(data, sig[:]...)
}

func buildPingMessage() []byte {
	return []byte{MessageTypePing}
}

func buildPongMessage() []byte {
	return []byte{MessageTypePong}
}

func buildSnapshotMessage(ss *common.Snapshot) []byte {
	data := common.MsgpackMarshalPanic(ss)
	return append([]byte{MessageTypeSnapshot}, data...)
}

func buildGraphMessage(g *RoundGraph) []byte {
	data := common.MsgpackMarshalPanic(g.FinalCache)
	return append([]byte{MessageTypeGraph}, data...)
}

func (node *Node) openPeerStream(peer *Peer) error {
	logger.Println("OPEN PEER STREAM", peer.Address)
	transport, err := network.NewQuicClient(peer.Address)
	if err != nil {
		return err
	}
	client, err := transport.Dial()
	if err != nil {
		return err
	}
	defer client.Close()
	logger.Println("DIAL PEER STREAM", peer.Address)

	err = client.Send(buildAuthenticationMessage(node.Account))
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
			err := client.Send(buildGraphMessage(node.Graph))
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

func (node *Node) acceptPeerStream(client network.Client) error {
	defer client.Close()

	peer, err := node.authenticatePeer(client)
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
		case MessageTypePing:
			err = client.Send(buildPongMessage())
			if err != nil {
				return err
			}
		case MessageTypeSnapshot:
			if msg.Snapshot.CheckSignature(peer.Account.PublicSpendKey) {
				node.feedMempool(msg.Snapshot)
			}
		case MessageTypeGraph:
			peer.GraphChan <- msg.FinalCache
		}
	}
}

func (node *Node) authenticatePeer(client network.Client) (*Peer, error) {
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
		if msg.Type != MessageTypeAuthentication {
			auth <- errors.New("peer authentication invalid message type")
			return
		}
		ts := binary.BigEndian.Uint64(msg.Data[:8])
		if time.Now().Unix()-int64(ts) > 3 {
			auth <- errors.New("peer authentication message timeout")
			return
		}
		for _, p := range node.GossipPeers {
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
