package kernel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/network"
	"github.com/vmihailenco/msgpack"
)

type Peer struct {
	Account     common.Address
	Address     string
	RoundNumber uint64
	RoundHash   crypto.Hash

	send chan []byte
}

const (
	MessageTypeSnapshot       = 0
	MessageTypePing           = 1
	MessageTypePong           = 2
	MessageTypeAuthentication = 3
)

type Message struct {
	Type     uint8
	Snapshot *common.Snapshot
	Data     []byte
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

func buildSnapshotMessage(ss *common.Snapshot) ([]byte, error) {
	data := common.MsgpackMarshalPanic(ss)
	return append([]byte{MessageTypeSnapshot}, data...), nil
}

func NewPeer(acc common.Address, addr string) *Peer {
	return &Peer{
		Account: acc,
		Address: addr,
		send:    make(chan []byte, 64),
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

func (node *Node) managePeerStream(peer *Peer) error {
	transport, err := network.NewQuicClient(peer.Address)
	if err != nil {
		return err
	}
	client, err := transport.Dial()
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.Send(buildAuthenticationMessage(node.Account))
	if err != nil {
		return err
	}

	go func() error {
		defer client.Close()

		for {
			data, err := client.Receive()
			if err != nil {
				return err
			}
			msg, err := parseNetworkMessage(data)
			if err != nil {
				return err
			}
			logger.Println("PEER", msg.Type)
		}
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-peer.send:
			err := client.Send(msg)
			if err != nil {
				return err
			}
		case <-ticker.C:
			err := client.Send(buildPingMessage())
			if err != nil {
				return err
			}
		}
	}
}

func (node *Node) authenticatePeer(client network.Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		data, err := client.Receive()
		if err != nil {
			client.Close()
			return
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			client.Close()
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
		for _, p := range node.Peers {
			hash := p.Account.Hash()
			if !bytes.Equal(hash[:], msg.Data[8:40]) {
				continue
			}
			var sig crypto.Signature
			copy(sig[:], msg.Data[40:])
			if p.Account.PublicSpendKey.Verify(msg.Data[:40], sig) {
				auth <- nil
				peer = p
				return
			}
			break
		}
		auth <- errors.New("peer authentication message signature invalid")
	}()

	select {
	case err := <-auth:
		if err != nil {
			return nil, fmt.Errorf("peer authentication failed %s", err.Error())
		}
	case <-time.After(3 * time.Second):
		client.Close()
		return nil, errors.New("peer authentication timeout")
	}
	return peer, nil
}
