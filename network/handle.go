package network

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/ristretto"
)

const (
	PeerMessageTypePing               = 1
	PeerMessageTypeAuthentication     = 3
	PeerMessageTypeGraph              = 4
	PeerMessageTypeSnapshotConfirm    = 5
	PeerMessageTypeTransactionRequest = 6
	PeerMessageTypeTransaction        = 7

	PeerMessageTypeSnapshotAnnouncement = 10 // leader send snapshot to peer
	PeerMessageTypeSnapshotCommitment   = 11 // peer generate ri based, send Ri to leader
	PeerMessageTypeTransactionChallenge = 12 // leader send bitmask Z and aggregated R to peer
	PeerMessageTypeSnapshotResponse     = 13 // peer generate A from nodes and Z, send response si = ri + H(R || A || M)ai to leader
	PeerMessageTypeSnapshotFinalization = 14 // leader generate A, verify si B = ri B + H(R || A || M)ai B = Ri + H(R || A || M)Ai, then finalize based on threshold
	PeerMessageTypeCommitments          = 15
	PeerMessageTypeFullChallenge        = 16

	PeerMessageTypeRelay     = 200
	PeerMessageTypeConsumers = 201

	MsgPriorityNormal = 0
	MsgPriorityHigh   = 1
)

type PeerMessage struct {
	Type            uint8
	Snapshot        *common.Snapshot
	SnapshotHash    crypto.Hash
	Transaction     *common.VersionedTransaction
	TransactionHash crypto.Hash
	Cosi            crypto.CosiSignature
	Commitment      crypto.Key
	Challenge       crypto.Key
	Response        [32]byte
	WantTx          bool
	Commitments     []*crypto.Key
	Graph           []*SyncPoint
	Data            []byte

	version byte
}

type AuthToken struct {
	PeerId    crypto.Hash
	Timestamp uint64
	IsRelayer bool
	Data      []byte
}

type SyncHandle interface {
	GetCacheStore() *ristretto.Cache
	BuildAuthenticationMessage(relayerId crypto.Hash) []byte
	AuthenticateAs(recipientId crypto.Hash, msg []byte, timeoutSec int64) (*AuthToken, error)
	BuildGraph() []*SyncPoint
	UpdateSyncPoint(peerId crypto.Hash, points []*SyncPoint)
	ReadAllNodesWithoutState() []crypto.Hash
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	SendTransactionToPeer(peerId, tx crypto.Hash) error
	CachePutTransaction(peerId crypto.Hash, ver *common.VersionedTransaction) error
	CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, R *crypto.Key) error
	CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTx bool) error
	CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error
	CosiQueueExternalFullChallenge(peerId crypto.Hash, s *common.Snapshot, commitment, challenge *crypto.Key, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error
	CosiAggregateSelfResponses(peerId crypto.Hash, snap crypto.Hash, response *[32]byte) error
	VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot) error
	CosiQueueExternalCommitments(peerId crypto.Hash, commitments []*crypto.Key) error
}

func (me *Peer) SendGraphMessage(idForNetwork crypto.Hash) error {
	msg := buildGraphMessage(me.handle.BuildGraph())
	return me.sendHighToPeer(idForNetwork, PeerMessageTypeGraph, nil, msg)
}

func (me *Peer) SendCommitmentsMessage(idForNetwork crypto.Hash, commitments []*crypto.Key) error {
	data := buildCommitmentsMessage(commitments)
	hash := crypto.Blake3Hash(data)
	key := append(idForNetwork[:], 'C', 'R')
	key = append(key, hash[:]...)
	return me.sendHighToPeer(idForNetwork, PeerMessageTypeCommitments, key, data)
}

func (me *Peer) SendSnapshotAnnouncementMessage(idForNetwork crypto.Hash, s *common.Snapshot, R crypto.Key) error {
	data := buildSnapshotAnnouncementMessage(s, R)
	return me.sendSnapshotMessageToPeer(idForNetwork, s.PayloadHash(), PeerMessageTypeSnapshotAnnouncement, data)
}

func (me *Peer) SendSnapshotCommitmentMessage(idForNetwork crypto.Hash, snap crypto.Hash, R crypto.Key, wantTx bool) error {
	data := buildSnapshotCommitmentMessage(snap, R, wantTx)
	return me.sendSnapshotMessageToPeer(idForNetwork, snap, PeerMessageTypeSnapshotCommitment, data)
}

func (me *Peer) SendTransactionChallengeMessage(idForNetwork crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, tx *common.VersionedTransaction) error {
	data := buildTransactionChallengeMessage(snap, cosi, tx)
	return me.sendSnapshotMessageToPeer(idForNetwork, snap, PeerMessageTypeTransactionChallenge, data)
}

func (me *Peer) SendFullChallengeMessage(idForNetwork crypto.Hash, s *common.Snapshot, commitment, challenge *crypto.Key, tx *common.VersionedTransaction) error {
	data := buildFullChanllengeMessage(s, commitment, challenge, tx)
	return me.sendSnapshotMessageToPeer(idForNetwork, s.PayloadHash(), PeerMessageTypeFullChallenge, data)
}

func (me *Peer) SendSnapshotResponseMessage(idForNetwork crypto.Hash, snap crypto.Hash, si *[32]byte) error {
	data := buildSnapshotResponseMessage(snap, si)
	return me.sendSnapshotMessageToPeer(idForNetwork, snap, PeerMessageTypeSnapshotResponse, data)
}

func (me *Peer) SendSnapshotFinalizationMessage(idForNetwork crypto.Hash, s *common.Snapshot) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}

	key := append(idForNetwork[:], s.Hash[:]...)
	key = append(key, 'S', 'C', 'O')
	if me.snapshotsCaches.contains(key, time.Hour) {
		return nil
	}

	data := buildSnapshotFinalizationMessage(s)
	return me.sendSnapshotMessageToPeer(idForNetwork, s.Hash, PeerMessageTypeSnapshotFinalization, data)
}

func (me *Peer) SendSnapshotConfirmMessage(idForNetwork crypto.Hash, snap crypto.Hash) error {
	key := append(idForNetwork[:], snap[:]...)
	key = append(key, 'S', 'N', 'A', 'P', PeerMessageTypeSnapshotConfirm)
	return me.sendHighToPeer(idForNetwork, PeerMessageTypeSnapshotConfirm, key, buildSnapshotConfirmMessage(snap))
}

func (me *Peer) SendTransactionRequestMessage(idForNetwork crypto.Hash, tx crypto.Hash) error {
	key := append(idForNetwork[:], tx[:]...)
	key = append(key, 'T', 'X', PeerMessageTypeTransactionRequest)
	return me.sendHighToPeer(idForNetwork, PeerMessageTypeTransactionRequest, key, buildTransactionRequestMessage(tx))
}

func (me *Peer) SendTransactionMessage(idForNetwork crypto.Hash, ver *common.VersionedTransaction) error {
	tx := ver.PayloadHash()
	key := append(idForNetwork[:], tx[:]...)
	key = append(key, 'T', 'X', PeerMessageTypeTransaction)
	return me.sendHighToPeer(idForNetwork, PeerMessageTypeTransaction, key, buildTransactionMessage(ver))
}

func (me *Peer) ConfirmSnapshotForPeer(idForNetwork, snap crypto.Hash) {
	key := append(idForNetwork[:], snap[:]...)
	key = append(key, 'S', 'C', 'O')
	me.snapshotsCaches.store(key, time.Now())
}

func buildAuthenticationMessage(data []byte) []byte {
	header := []byte{PeerMessageTypeAuthentication}
	return append(header, data...)
}

func buildSnapshotAnnouncementMessage(s *common.Snapshot, R crypto.Key) []byte {
	data := s.VersionedMarshal()
	data = append(R[:], data...)
	return append([]byte{PeerMessageTypeSnapshotAnnouncement}, data...)
}

func buildSnapshotCommitmentMessage(snap crypto.Hash, R crypto.Key, wantTx bool) []byte {
	data := []byte{PeerMessageTypeSnapshotCommitment}
	data = append(data, snap[:]...)
	data = append(data, R[:]...)
	if wantTx {
		return append(data, byte(1))
	}
	return append(data, byte(0))
}

func buildTransactionChallengeMessage(snap crypto.Hash, cosi *crypto.CosiSignature, tx *common.VersionedTransaction) []byte {
	data := []byte{PeerMessageTypeTransactionChallenge}
	data = append(data, snap[:]...)
	data = append(data, cosi.Signature[:]...)
	data = binary.BigEndian.AppendUint64(data, cosi.Mask)
	if tx != nil {
		pl := tx.Marshal()
		return append(data, pl...)
	}
	return data
}

func buildFullChanllengeMessage(s *common.Snapshot, commitment, challenge *crypto.Key, tx *common.VersionedTransaction) []byte {
	data := []byte{PeerMessageTypeFullChallenge}

	pl := s.VersionedMarshal()
	data = binary.BigEndian.AppendUint32(data, uint32(len(pl)))
	data = append(data, pl[:]...)

	data = append(data, commitment[:]...)
	data = append(data, challenge[:]...)

	pl = tx.Marshal()
	data = binary.BigEndian.AppendUint32(data, uint32(len(pl)))
	return append(data, pl...)
}

func buildSnapshotResponseMessage(snap crypto.Hash, si *[32]byte) []byte {
	data := []byte{PeerMessageTypeSnapshotResponse}
	data = append(data, snap[:]...)
	return append(data, si[:]...)
}

func buildSnapshotFinalizationMessage(s *common.Snapshot) []byte {
	data := s.VersionedMarshal()
	return append([]byte{PeerMessageTypeSnapshotFinalization}, data...)
}

func buildSnapshotConfirmMessage(snap crypto.Hash) []byte {
	return append([]byte{PeerMessageTypeSnapshotConfirm}, snap[:]...)
}

func buildTransactionMessage(ver *common.VersionedTransaction) []byte {
	data := ver.Marshal()
	return append([]byte{PeerMessageTypeTransaction}, data...)
}

func buildTransactionRequestMessage(tx crypto.Hash) []byte {
	return append([]byte{PeerMessageTypeTransactionRequest}, tx[:]...)
}

func buildGraphMessage(points []*SyncPoint) []byte {
	data := marshalSyncPoints(points)
	return append([]byte{PeerMessageTypeGraph}, data...)
}

func buildCommitmentsMessage(commitments []*crypto.Key) []byte {
	if len(commitments) > 1024 {
		panic(len(commitments))
	}
	data := []byte{PeerMessageTypeCommitments}
	data = binary.BigEndian.AppendUint16(data, uint16(len(commitments)))
	for _, k := range commitments {
		data = append(data, k[:]...)
	}
	return data
}

func (me *Peer) buildConsumersMessage() []byte {
	data := []byte{PeerMessageTypeConsumers}
	peers := me.consumers.Slice()
	for _, p := range peers {
		data = append(data, p.IdForNetwork[:]...)
		data = append(data, p.consumerAuth.Data...)
	}
	return data
}

func (me *Peer) buildRelayMessage(peerId crypto.Hash, msg []byte) []byte {
	data := []byte{PeerMessageTypeRelay}
	data = append(data, me.IdForNetwork[:]...)
	data = append(data, peerId[:]...)
	if len(msg) > TransportMessageMaxSize {
		panic(hex.EncodeToString(msg))
	}
	data = append(data, msg...)
	return data
}

func parseNetworkMessage(version uint8, data []byte) (*PeerMessage, error) {
	if len(data) < 1 {
		return nil, errors.New("invalid message data")
	}
	msg := &PeerMessage{Type: data[0], version: version}
	switch msg.Type {
	case PeerMessageTypeCommitments:
		if len(data) < 3 {
			return nil, fmt.Errorf("invalid commitments message size %d", len(data))
		}
		count := binary.BigEndian.Uint16(data[1:3])
		if count > 1024 {
			return nil, fmt.Errorf("too much commitments %d", count)
		}
		if len(data[3:]) != int(count)*32 {
			return nil, fmt.Errorf("malformed commitments message %d %d", count, len(data[3:]))
		}
		for i := uint16(0); i < count; i++ {
			var key crypto.Key
			copy(key[:], data[3+32*i:])
			msg.Commitments = append(msg.Commitments, &key)
		}
	case PeerMessageTypeGraph:
		points, err := unmarshalSyncPoints(data[1:])
		if err != nil {
			return nil, err
		}
		msg.Graph = points
	case PeerMessageTypePing:
	case PeerMessageTypeAuthentication:
		msg.Data = data[1:]
	case PeerMessageTypeSnapshotConfirm:
		copy(msg.SnapshotHash[:], data[1:])
	case PeerMessageTypeTransaction:
		ver, err := common.UnmarshalVersionedTransaction(data[1:])
		if err != nil {
			return nil, err
		}
		msg.Transaction = ver
	case PeerMessageTypeTransactionRequest:
		copy(msg.TransactionHash[:], data[1:])
	case PeerMessageTypeSnapshotAnnouncement:
		if len(data[1:]) <= 32 {
			return nil, fmt.Errorf("invalid announcement message size %d", len(data[1:]))
		}
		copy(msg.Commitment[:], data[1:])
		snap, err := common.UnmarshalVersionedSnapshot(data[33:])
		if err != nil {
			return nil, err
		}
		if snap == nil {
			return nil, fmt.Errorf("invalid snapshot announcement message data")
		}
		msg.Snapshot = snap.Snapshot
	case PeerMessageTypeSnapshotCommitment:
		if len(data[1:]) != 65 {
			return nil, fmt.Errorf("invalid commitment message size %d", len(data[1:]))
		}
		copy(msg.SnapshotHash[:], data[1:])
		copy(msg.Commitment[:], data[33:])
		msg.WantTx = data[65] == 1
	case PeerMessageTypeFullChallenge:
		if len(data[1:]) < 256 {
			return nil, fmt.Errorf("invalid full challenge message size %d", len(data[1:]))
		}
		offset := 1 + 4
		size := int(binary.BigEndian.Uint32(data[1:offset]))
		if len(data[offset:]) < size {
			return nil, fmt.Errorf("invalid full challenge snapshot size %d %d", len(data[offset:]), size)
		}
		s, err := common.UnmarshalVersionedSnapshot(data[offset : offset+size])
		if err != nil {
			return nil, fmt.Errorf("invalid full challenge snapshot %v", err)
		}
		msg.Snapshot = s.Snapshot
		offset = offset + size
		if len(data[offset:]) < 256 {
			return nil, fmt.Errorf("invalid full challenge message size %d %d", offset, len(data[offset:]))
		}
		msg.Cosi = *s.Snapshot.Signature
		msg.Snapshot.Signature = nil

		copy(msg.Commitment[:], data[offset:offset+32])
		offset = offset + 32
		copy(msg.Challenge[:], data[offset:offset+32])
		offset = offset + 32

		size = int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if len(data[offset:]) < size {
			return nil, fmt.Errorf("invalid full challenge transaction size %d %d", len(data[offset:]), size)
		}
		offset = offset + 4
		ver, err := common.UnmarshalVersionedTransaction(data[offset : offset+size])
		if err != nil {
			return nil, fmt.Errorf("invalid full challenge transaction %v", err)
		}
		msg.Transaction = ver
	case PeerMessageTypeTransactionChallenge:
		if len(data[1:]) < 104 {
			return nil, fmt.Errorf("invalid transaction challenge message size %d", len(data[1:]))
		}
		copy(msg.SnapshotHash[:], data[1:])
		copy(msg.Cosi.Signature[:], data[33:])
		msg.Cosi.Mask = binary.BigEndian.Uint64(data[97:105])
		if len(data[1:]) > 104 {
			ver, err := common.UnmarshalVersionedTransaction(data[105:])
			if err != nil {
				return nil, err
			}
			msg.Transaction = ver
		}
	case PeerMessageTypeSnapshotResponse:
		if len(data[1:]) != 64 {
			return nil, fmt.Errorf("invalid response message size %d", len(data[1:]))
		}
		copy(msg.SnapshotHash[:], data[1:])
		copy(msg.Response[:], data[33:])
	case PeerMessageTypeSnapshotFinalization:
		snap, err := common.UnmarshalVersionedSnapshot(data[1:])
		if err != nil {
			return nil, err
		}
		if snap == nil {
			return nil, fmt.Errorf("invalid snapshot finalization message data")
		}
		msg.Snapshot = snap.Snapshot
	case PeerMessageTypeRelay:
		msg.Data = data[1:]
	case PeerMessageTypeConsumers:
		msg.Data = data[1:]
	}
	return msg, nil
}

func (me *Peer) loopHandlePeerMessage(peerId crypto.Hash, receive chan *PeerMessage) {
	for msg := range receive {
		me.handlePeerMessage(peerId, msg)
	}
}

func (me *Peer) relayOrHandlePeerMessage(relayerId crypto.Hash, msg *PeerMessage) error {
	logger.Printf("me.relayOrHandlePeerMessage(%s, %s) => %s %v", me.Address, me.IdForNetwork, relayerId, msg.Data)
	if len(msg.Data) < 64 {
		return nil
	}
	var from, to crypto.Hash
	copy(from[:], msg.Data[:32])
	copy(to[:], msg.Data[32:64])
	if to == me.IdForNetwork {
		rm, err := parseNetworkMessage(msg.version, msg.Data[64:])
		logger.Printf("me.relayOrHandlePeerMessage.ME(%s, %s) => %s %v %v", me.Address, me.IdForNetwork, from, rm, err)
		if err != nil {
			return err
		}
		// FIXME check the relayed message signature from the actual sending peer
		// no need to do special check here, just ensure all message types has the
		// authentic signature, most already, a few needs improvement
		return me.handlePeerMessage(from, rm)
	}
	if me.relayer == nil {
		return nil
	}
	data := append([]byte{PeerMessageTypeRelay}, msg.Data...)
	peer := me.consumers.Get(to)
	if peer == nil {
		peer = me.relayers.Get(to)
	}
	if peer != nil {
		rk := crypto.Blake3Hash(append(msg.Data, to[:]...))
		rk = crypto.Blake3Hash(append(rk[:], relayerId[:]...))
		success, _ := peer.offer(MsgPriorityHigh, &ChanMsg{rk[:], data})
		if !success {
			return fmt.Errorf("peer send high timeout")
		}
		return nil
	}
	peer = me.remoteRelayers.Get(to)
	if peer == nil || peer.IdForNetwork == relayerId {
		return nil
	}
	rk := crypto.Blake3Hash(append(msg.Data, to[:]...))
	rk = crypto.Blake3Hash(append(rk[:], relayerId[:]...))
	success, _ := peer.offer(MsgPriorityHigh, &ChanMsg{rk[:], data})
	if !success {
		return fmt.Errorf("peer send high timeout")
	}
	return nil
}

func (me *Peer) updateRemoteRelayerConsumers(relayerId crypto.Hash, data []byte) error {
	logger.Verbosef("me.updateRemoteRelayerConsumers(%s, %s) => %x", me.Address, relayerId, data)
	relayer := me.relayers.Get(relayerId)
	if relayer == nil {
		relayer = me.consumers.Get(relayerId)
	}
	if relayer == nil || !relayer.isRemoteRelayer {
		return nil
	}
	pl := len(crypto.Key{}) + 137
	for c := len(data) / pl; c > 0; c-- {
		var id crypto.Hash
		copy(id[:], data[:32])
		token, err := me.handle.AuthenticateAs(relayerId, data[32:pl], 0)
		if err != nil {
			return nil
		}
		if token.PeerId != id {
			return nil
		}
		old := me.remoteRelayers.Get(id)
		if old == nil || old.consumerAuth == nil || old.consumerAuth.Timestamp < token.Timestamp {
			me.remoteRelayers.Set(id, relayer)
		}
		data = data[pl:]
	}
	return nil
}

func (me *Peer) handlePeerMessage(peerId crypto.Hash, msg *PeerMessage) error {
	switch msg.Type {
	case PeerMessageTypeRelay:
		return me.relayOrHandlePeerMessage(peerId, msg)
	case PeerMessageTypeConsumers:
		return me.updateRemoteRelayerConsumers(peerId, msg.Data)
	case PeerMessageTypePing:
	case PeerMessageTypeCommitments:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeCommitments %s %d\n", peerId, len(msg.Commitments))
		return me.handle.CosiQueueExternalCommitments(peerId, msg.Commitments)
	case PeerMessageTypeGraph:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeGraph %s\n", peerId)
		me.handle.UpdateSyncPoint(peerId, msg.Graph)
		peer := me.relayers.Get(peerId)
		if peer == nil {
			peer = me.consumers.Get(peerId)
		}
		if peer != nil {
			peer.syncRing.Offer(msg.Graph)
		}
		return nil
	case PeerMessageTypeTransactionRequest:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeTransactionRequest %s %s\n", peerId, msg.TransactionHash)
		return me.handle.SendTransactionToPeer(peerId, msg.TransactionHash)
	case PeerMessageTypeTransaction:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeTransaction %s\n", peerId)
		return me.handle.CachePutTransaction(peerId, msg.Transaction)
	case PeerMessageTypeSnapshotConfirm:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeSnapshotConfirm %s %s\n", peerId, msg.SnapshotHash)
		me.ConfirmSnapshotForPeer(peerId, msg.SnapshotHash)
		return nil
	case PeerMessageTypeSnapshotAnnouncement:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeSnapshotAnnouncement %s %s\n", peerId, msg.Snapshot.SoleTransaction())
		return me.handle.CosiQueueExternalAnnouncement(peerId, msg.Snapshot, &msg.Commitment)
	case PeerMessageTypeSnapshotCommitment:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeSnapshotCommitment %s %s\n", peerId, msg.SnapshotHash)
		return me.handle.CosiAggregateSelfCommitments(peerId, msg.SnapshotHash, &msg.Commitment, msg.WantTx)
	case PeerMessageTypeTransactionChallenge:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeTransactionChallenge %s %s %t\n", peerId, msg.SnapshotHash, msg.Transaction != nil)
		return me.handle.CosiQueueExternalChallenge(peerId, msg.SnapshotHash, &msg.Cosi, msg.Transaction)
	case PeerMessageTypeFullChallenge:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeFullChallenge %s %v %v\n", peerId, msg.Snapshot, msg.Transaction)
		return me.handle.CosiQueueExternalFullChallenge(peerId, msg.Snapshot, &msg.Commitment, &msg.Challenge, &msg.Cosi, msg.Transaction)
	case PeerMessageTypeSnapshotResponse:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeSnapshotResponse %s %s\n", peerId, msg.SnapshotHash)
		return me.handle.CosiAggregateSelfResponses(peerId, msg.SnapshotHash, &msg.Response)
	case PeerMessageTypeSnapshotFinalization:
		logger.Verbosef("network.handle handlePeerMessage PeerMessageTypeSnapshotFinalization %s %s\n", peerId, msg.Snapshot.SoleTransaction())
		return me.handle.VerifyAndQueueAppendSnapshotFinalization(peerId, msg.Snapshot)
	}
	return nil
}

func marshalSyncPoints(points []*SyncPoint) []byte {
	enc := common.NewMinimumEncoder()
	enc.WriteInt(len(points))
	for _, p := range points {
		enc.Write(p.NodeId[:])
		enc.WriteUint64(p.Number)
		enc.Write(p.Hash[:])
	}
	return enc.Bytes()
}

func unmarshalSyncPoints(b []byte) ([]*SyncPoint, error) {
	dec, err := common.NewMinimumDecoder(b)
	if err != nil {
		return nil, err
	}
	count, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	points := make([]*SyncPoint, count)
	for i := range points {
		p := &SyncPoint{}
		err = dec.Read(p.NodeId[:])
		if err != nil {
			return nil, err
		}
		num, err := dec.ReadUint64()
		if err != nil {
			return nil, err
		}
		p.Number = num
		err = dec.Read(p.Hash[:])
		if err != nil {
			return nil, err
		}
		points[i] = p
	}
	return points, nil
}

func marshalPeers(peers []string) []byte {
	enc := common.NewMinimumEncoder()
	enc.WriteInt(len(peers))
	for _, p := range peers {
		enc.WriteInt(len(p))
		enc.Write([]byte(p))
	}
	return enc.Bytes()
}

func unmarshalPeers(b []byte) ([]string, error) {
	dec, err := common.NewMinimumDecoder(b)
	if err != nil {
		return nil, err
	}
	count, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	peers := make([]string, count)
	for i := range peers {
		as, err := dec.ReadInt()
		if err != nil {
			return nil, err
		}
		addr := make([]byte, as)
		err = dec.Read(addr)
		if err != nil {
			return nil, err
		}
		peers[i] = string(addr)
	}
	return peers, nil
}
