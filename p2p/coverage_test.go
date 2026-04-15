package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/stretchr/testify/require"
)

func TestMapAndMetricHelpers(t *testing.T) {
	require := require.New(t)

	cache := newP2PTestCache(t)
	confirm := &confirmMap{cache: cache}
	require.False(confirm.contains(nil, time.Second))

	key := []byte("snapshot-key")
	require.False(confirm.contains(key, time.Second))
	confirm.store(key, time.Now())
	cache.Wait()
	require.True(confirm.contains(key, time.Second))
	require.False(confirm.contains(key, -time.Second))
	require.Panics(func() {
		confirm.store(nil, time.Now())
	})

	peerID := crypto.Blake3Hash([]byte("peer"))
	relayerID := crypto.Blake3Hash([]byte("relayer"))
	otherRelayerID := crypto.Blake3Hash([]byte("other-relayer"))
	relayers := &relayersMap{m: map[crypto.Hash][]*remoteRelayer{
		peerID: {
			{Id: otherRelayerID, ActiveAt: time.Now().Add(-2 * time.Minute)},
		},
	}}
	relayers.Add(peerID, relayerID)
	require.Equal([]crypto.Hash{relayerID}, relayers.Get(peerID))
	relayers.Add(peerID, relayerID)
	require.Equal([]crypto.Hash{relayerID}, relayers.Get(peerID))

	neighbors := &neighborMap{m: make(map[crypto.Hash]*Peer)}
	peer := NewPeer(nil, relayerID, "127.0.0.1:9001", true)
	require.True(neighbors.Put(relayerID, peer))
	require.False(neighbors.Put(relayerID, peer))
	require.Equal(peer, neighbors.Get(relayerID))
	require.Len(neighbors.Slice(), 1)
	neighbors.Delete(relayerID)
	require.Nil(neighbors.Get(relayerID))
	neighbors.Set(relayerID, peer)
	require.Equal(peer, neighbors.Get(relayerID))

	mp := &MetricPool{enabled: true}
	for _, typ := range []uint8{
		PeerMessageTypePing,
		PeerMessageTypeAuthentication,
		PeerMessageTypeGraph,
		PeerMessageTypeSnapshotConfirm,
		PeerMessageTypeTransactionRequest,
		PeerMessageTypeTransaction,
		PeerMessageTypeSnapshotAnnouncement,
		PeerMessageTypeSnapshotCommitment,
		PeerMessageTypeTransactionChallenge,
		PeerMessageTypeSnapshotResponse,
		PeerMessageTypeSnapshotFinalization,
		PeerMessageTypeCommitments,
		PeerMessageTypeFullChallenge,
		PeerMessageTypeRelay,
	} {
		mp.handle(typ)
	}
	require.Equal(uint32(1), mp.PeerMessageTypePing)
	require.Equal(uint32(1), mp.PeerMessageTypeRelay)
	require.Contains(mp.String(), `"relay":1`)

	me := NewPeer(nil, crypto.Blake3Hash([]byte("me")), "127.0.0.1:9002", false)
	me.sentMetric.enabled = true
	me.receivedMetric.enabled = true
	metrics := me.Metric()
	require.Contains(metrics, "sent")
	require.Contains(metrics, "received")
}

func TestBuildAndParseNetworkMessages(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	tx := p2pTestTransaction()
	fullTx := common.NewTransactionV5(common.XINAssetId)
	fullTx.Extra = bytes.Repeat([]byte{4}, 220)
	fullVer := fullTx.AsVersioned()
	snapshot := p2pTestSnapshot(true)
	commitment := p2pTestPrivateKey(11).Public()
	challenge := p2pTestPrivateKey(12).Public()
	spend := p2pTestPrivateKey(13)

	msg, err := parseNetworkMessage(7, buildAuthenticationMessage([]byte("auth-data")))
	require.Nil(err)
	require.EqualValues(PeerMessageTypeAuthentication, msg.Type)
	require.Equal([]byte("auth-data"), msg.Data)

	msg, err = parseNetworkMessage(7, buildSnapshotConfirmMessage(snapshot.PayloadHash()))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.SnapshotHash)

	msg, err = parseNetworkMessage(7, buildTransactionRequestMessage(tx.PayloadHash()))
	require.Nil(err)
	require.Equal(tx.PayloadHash(), msg.TransactionHash)

	msg, err = parseNetworkMessage(7, buildTransactionMessage(tx))
	require.Nil(err)
	require.Len(msg.Transactions, 1)
	require.Equal(tx.PayloadHash(), msg.Transactions[0].PayloadHash())

	msg, err = parseNetworkMessage(7, buildSnapshotAnnouncementMessage(snapshot, commitment, spend))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.Snapshot.PayloadHash())
	require.Equal(commitment, msg.Commitment)
	require.NotNil(msg.signature)

	msg, err = parseNetworkMessage(7, buildSnapshotCommitmentMessage(handle, snapshot.PayloadHash(), commitment, []crypto.Hash{tx.PayloadHash()}))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.SnapshotHash)
	require.Equal(commitment, msg.Commitment)
	require.Equal([]crypto.Hash{tx.PayloadHash()}, msg.WantTxs)
	require.NotNil(msg.signature)

	cosi := &crypto.CosiSignature{Mask: 3}
	msg, err = parseNetworkMessage(7, buildTransactionChallengeMessage(snapshot.PayloadHash(), cosi, nil))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.SnapshotHash)
	require.Equal(cosi.Mask, msg.Cosi.Mask)
	require.Empty(msg.Transactions)

	msg, err = parseNetworkMessage(7, buildTransactionChallengeMessage(snapshot.PayloadHash(), cosi, []*common.VersionedTransaction{tx}))
	require.Nil(err)
	require.Len(msg.Transactions, 1)
	require.Equal(tx.PayloadHash(), msg.Transactions[0].PayloadHash())

	msg, err = parseNetworkMessage(7, buildSnapshotResponseMessage(snapshot.PayloadHash(), &[32]byte{1, 2, 3}))
	require.Nil(err)
	require.Equal(byte(1), msg.Response[0])

	msg, err = parseNetworkMessage(7, buildSnapshotFinalizationMessage(snapshot))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.Snapshot.PayloadHash())

	msg, err = parseNetworkMessage(7, buildGraphMessage(handle))
	require.Nil(err)
	require.Len(msg.Graph, 1)
	require.NotNil(msg.signature)
	require.Equal(handle.graph[0].NodeId, msg.Graph[0].NodeId)

	commitments := []*crypto.Key{&commitment, &challenge}
	msg, err = parseNetworkMessage(7, buildCommitmentsMessage(handle, commitments))
	require.Nil(err)
	require.Len(msg.Commitments, 2)
	require.NotNil(msg.signature)

	msg, err = parseNetworkMessage(7, buildFullChallengeMessage(snapshot, &commitment, &challenge, []*common.VersionedTransaction{fullVer}))
	require.Nil(err)
	require.Equal(snapshot.PayloadHash(), msg.Snapshot.PayloadHash())
	require.Equal(snapshot.Signature.Mask, msg.Cosi.Mask)
	require.Nil(msg.Snapshot.Signature)
	require.Len(msg.Transactions, 1)
	require.Equal(fullVer.PayloadHash(), msg.Transactions[0].PayloadHash())

	me := NewPeer(handle, crypto.Blake3Hash([]byte("relay-me")), "127.0.0.1:9003", true)
	consumerID := crypto.Blake3Hash([]byte("consumer"))
	consumer := NewPeer(nil, consumerID, "127.0.0.1:9004", false)
	consumer.consumerAuth = &AuthToken{Data: bytes.Repeat([]byte{9}, 137)}
	me.consumers.Set(consumerID, consumer)

	msg, err = parseNetworkMessage(7, me.buildConsumersMessage())
	require.Nil(err)
	require.EqualValues(PeerMessageTypeConsumers, msg.Type)
	require.Len(msg.Data, 32+137)

	relayPayload := me.buildRelayMessage(consumerID, buildTransactionRequestMessage(tx.PayloadHash()))
	msg, err = parseNetworkMessage(7, relayPayload)
	require.Nil(err)
	require.EqualValues(PeerMessageTypeRelay, msg.Type)
	require.Equal(relayPayload, msg.Data)

	points, err := unmarshalSyncPoints(marshalSyncPoints(handle.graph))
	require.Nil(err)
	require.Len(points, 1)
	require.Equal(handle.graph[0].Hash, points[0].Hash)

	_, err = parseNetworkMessage(7, nil)
	require.ErrorContains(err, "invalid message data")
	_, err = parseNetworkMessage(7, []byte{PeerMessageTypeSnapshotResponse, 1})
	require.ErrorContains(err, "invalid response message size")
}

func TestP2PMessageAndPeerEdgeCases(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	tx := p2pTestTransaction()
	snapshot := p2pTestSnapshot(true)
	commitment := p2pTestPrivateKey(61).Public()
	challenge := p2pTestPrivateKey(62).Public()
	fullTx := common.NewTransactionV5(common.XINAssetId)
	fullTx.Extra = bytes.Repeat([]byte{4}, 220)
	fullVer := fullTx.AsVersioned()

	msg, err := parseNetworkMessage(7, buildSnapshotCommitmentMessage(handle, snapshot.PayloadHash(), commitment, nil))
	require.Nil(err)
	require.Empty(msg.WantTxs)

	shortCommitments := make([]byte, 80)
	shortCommitments[0] = PeerMessageTypeCommitments
	binary.BigEndian.PutUint16(shortCommitments[65:67], 1025)
	_, err = parseNetworkMessage(7, shortCommitments)
	require.ErrorContains(err, "too much commitments")

	malformedCommitments := make([]byte, 80)
	malformedCommitments[0] = PeerMessageTypeCommitments
	binary.BigEndian.PutUint16(malformedCommitments[65:67], 1)
	_, err = parseNetworkMessage(7, malformedCommitments)
	require.ErrorContains(err, "malformed commitments")

	_, err = parseNetworkMessage(7, buildGraphMessage(handle)[:70])
	require.Error(err)
	_, err = unmarshalSyncPoints([]byte{1})
	require.Error(err)

	_, err = parseNetworkMessage(7, bytes.Repeat([]byte{PeerMessageTypeSnapshotAnnouncement}, 10))
	require.ErrorContains(err, "invalid announcement message size")
	badAnnouncement := append([]byte{PeerMessageTypeSnapshotAnnouncement}, bytes.Repeat([]byte{1}, 100)...)
	_, err = parseNetworkMessage(7, badAnnouncement)
	require.Error(err)

	_, err = parseNetworkMessage(7, bytes.Repeat([]byte{PeerMessageTypeFullChallenge}, 10))
	require.ErrorContains(err, "invalid full challenge message size")
	full := buildFullChallengeMessage(snapshot, &commitment, &challenge, []*common.VersionedTransaction{fullVer})
	badFullSnapshot := append([]byte{}, full...)
	binary.BigEndian.PutUint32(badFullSnapshot[1:5], 1<<20)
	_, err = parseNetworkMessage(7, badFullSnapshot)
	require.ErrorContains(err, "invalid full challenge snapshot size")
	badFullTx := append([]byte{}, full...)
	offset := 1 + 4 + len(snapshot.VersionedMarshal()) + 32 + 32 + 1
	binary.BigEndian.PutUint32(badFullTx[offset:offset+4], 1<<20)
	_, err = parseNetworkMessage(7, badFullTx)
	require.ErrorContains(err, "invalid transactions payload size")

	_, err = parseNetworkMessage(7, []byte{PeerMessageTypeTransactionChallenge, 1})
	require.ErrorContains(err, "invalid transaction challenge message size")
	badChallenge := append(buildTransactionChallengeMessage(snapshot.PayloadHash(), &crypto.CosiSignature{Mask: 1}, nil), 0xff)
	_, err = parseNetworkMessage(7, badChallenge)
	require.Error(err)

	_, err = parseNetworkMessage(7, []byte{PeerMessageTypeSnapshotFinalization, 0})
	require.Error(err)

	manyCommitments := make([]*crypto.Key, 1025)
	for i := range manyCommitments {
		manyCommitments[i] = &commitment
	}
	require.Panics(func() {
		buildCommitmentsMessage(handle, manyCommitments)
	})

	me := NewPeer(handle, crypto.Blake3Hash([]byte("edge-relay")), "127.0.0.1:0", true)
	require.Panics(func() {
		me.buildRelayMessage(crypto.Blake3Hash([]byte("edge-target")), bytes.Repeat([]byte{1}, TransportMessageMaxSize+1))
	})

	nonRelayer := NewPeer(handle, crypto.Blake3Hash([]byte("non-relay")), "127.0.0.1:0", false)
	err = nonRelayer.updateRemoteRelayerConsumers(crypto.Blake3Hash([]byte("relayer")), bytes.Repeat([]byte{1}, 169))
	require.Nil(err)

	relayer := NewPeer(handle, crypto.Blake3Hash([]byte("relay")), "127.0.0.1:0", true)
	relayer.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	consumerID := crypto.Blake3Hash([]byte("consumer"))
	data := append(consumerID[:], bytes.Repeat([]byte{7}, 137)...)
	handle.authErr = errors.New("authenticate failed")
	require.Panics(func() {
		relayer.updateRemoteRelayerConsumers(relayer.IdForNetwork, data)
	})
	handle.authErr = nil
	handle.authToken = &AuthToken{PeerId: crypto.Blake3Hash([]byte("other-consumer"))}
	require.Panics(func() {
		relayer.updateRemoteRelayerConsumers(relayer.IdForNetwork, data)
	})

	handle.authToken = &AuthToken{PeerId: crypto.Blake3Hash([]byte("graph-consumer"))}
	handle.updateErr = errors.New("update failed")
	err = me.handlePeerMessage(crypto.Blake3Hash([]byte("peer")), &PeerMessage{
		Type:      PeerMessageTypeGraph,
		Graph:     handle.graph,
		unsigned:  []byte("graph"),
		signature: func() *crypto.Signature { sig := handle.SignData([]byte("graph")); return &sig }(),
	})
	require.ErrorIs(err, handle.updateErr)
	handle.updateErr = nil

	err = me.handlePeerMessage(crypto.Blake3Hash([]byte("peer")), &PeerMessage{Type: PeerMessageTypePing})
	require.Nil(err)
	err = me.handlePeerMessage(crypto.Blake3Hash([]byte("peer")), &PeerMessage{Type: 255})
	require.Nil(err)

	listenErr := NewPeer(handle, crypto.Blake3Hash([]byte("bad-listen")), "bad-addr", true).ListenConsumers()
	require.Error(listenErr)

	sender := NewPeer(handle, crypto.Blake3Hash([]byte("sender-edge")), "127.0.0.1:0", true)
	sender.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	target := crypto.Blake3Hash([]byte("send-target"))
	relayID := crypto.Blake3Hash([]byte("relay-id"))
	sender.remoteRelayers.Add(target, relayID)
	sender.relayers.Set(relayID, NewPeer(nil, relayID, "127.0.0.1:0", false))
	err = sender.sendToPeer(sender.IdForNetwork, PeerMessageTypePing, nil, []byte("noop"), MsgPriorityNormal)
	require.Nil(err)
	cachedKey := []byte("edge-cached")
	sender.snapshotsCaches.store(cachedKey, time.Now())
	sender.snapshotsCaches.cache.Wait()
	err = sender.sendToPeer(target, PeerMessageTypePing, cachedKey, []byte("noop"), MsgPriorityNormal)
	require.Nil(err)
	require.Nil(NewPeer(handle, crypto.Blake3Hash([]byte("no-remote")), "127.0.0.1:0", true).GetRemoteRelayers(target))
	require.Panics(func() {
		sender.sendToPeer(target, PeerMessageTypeTransactionRequest, nil, buildTransactionRequestMessage(tx.PayloadHash()), MsgPriorityNormal)
	})

	fallback := NewPeer(handle, crypto.Blake3Hash([]byte("fallback-sender")), "127.0.0.1:0", true)
	relayPeer := NewPeer(nil, crypto.Blake3Hash([]byte("fallback-relay")), "127.0.0.1:0", true)
	fallback.relayers.Set(relayPeer.IdForNetwork, relayPeer)
	err = fallback.sendToPeer(target, PeerMessageTypePing, nil, []byte("fallback"), MsgPriorityNormal)
	require.Nil(err)
	select {
	case offered := <-relayPeer.normalRing:
		require.Equal([]byte("fallback"), offered.data[65:])
	default:
		t.Fatal("expected relay fallback message")
	}
}

func TestHandlePeerMessageDispatch(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("self")), "127.0.0.1:9010", true)
	me.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	peerID := crypto.Blake3Hash([]byte("peer-id"))
	snap := p2pTestSnapshot(true)
	tx := p2pTestTransaction()
	commitment := p2pTestPrivateKey(21).Public()
	challenge := p2pTestPrivateKey(22).Public()
	sig := handle.SignData([]byte("dispatch"))
	response := [32]byte{7, 8, 9}

	neighbor := NewPeer(nil, peerID, "127.0.0.1:9011", false)
	me.relayers.Set(peerID, neighbor)

	err := me.handlePeerMessage(peerID, &PeerMessage{
		Type:        PeerMessageTypeCommitments,
		Commitments: []*crypto.Key{&commitment},
		unsigned:    []byte("commitments"),
		signature:   &sig,
	})
	require.Nil(err)
	require.Len(handle.lastCommitments, 1)

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:      PeerMessageTypeGraph,
		Graph:     handle.graph,
		unsigned:  []byte("graph"),
		signature: &sig,
	})
	require.Nil(err)
	select {
	case graph := <-neighbor.syncRing:
		require.Len(graph, 1)
	default:
		t.Fatal("expected graph sync message")
	}

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:            PeerMessageTypeTransactionRequest,
		TransactionHash: tx.PayloadHash(),
	})
	require.Nil(err)
	require.Equal(tx.PayloadHash(), handle.requestedTx)

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeTransaction,
		Transactions: []*common.VersionedTransaction{tx},
	})
	require.Nil(err)
	require.Equal(tx.PayloadHash(), handle.cachedTx.PayloadHash())

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeSnapshotConfirm,
		SnapshotHash: snap.PayloadHash(),
	})
	require.Nil(err)
	me.snapshotsCaches.cache.Wait()
	snapHash := snap.PayloadHash()
	confirmKey := append(peerID[:], snapHash[:]...)
	confirmKey = append(confirmKey, 'S', 'C', 'O')
	require.True(me.snapshotsCaches.contains(confirmKey, time.Hour))

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:       PeerMessageTypeSnapshotAnnouncement,
		Snapshot:   snap,
		Commitment: commitment,
		signature:  &sig,
	})
	require.Nil(err)
	require.Equal(snap.PayloadHash(), handle.announcement.PayloadHash())

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeSnapshotCommitment,
		SnapshotHash: snap.PayloadHash(),
		Commitment:   commitment,
		WantTxs:      []crypto.Hash{tx.PayloadHash()},
		unsigned:     []byte("unsigned-commitment"),
		signature:    &sig,
	})
	require.Nil(err)
	require.Equal([]crypto.Hash{tx.PayloadHash()}, handle.wantTxs)

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeTransactionChallenge,
		SnapshotHash: snap.PayloadHash(),
		Cosi:         crypto.CosiSignature{Mask: 5},
		Transactions: []*common.VersionedTransaction{tx},
	})
	require.Nil(err)
	require.Len(handle.challengeTxs, 1)
	require.Equal(tx.PayloadHash(), handle.challengeTxs[0].PayloadHash())

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeFullChallenge,
		Snapshot:     snap,
		Commitment:   commitment,
		Challenge:    challenge,
		Cosi:         crypto.CosiSignature{Mask: 6},
		Transactions: []*common.VersionedTransaction{tx},
	})
	require.Nil(err)
	require.Len(handle.fullChallengeTxs, 1)
	require.Equal(tx.PayloadHash(), handle.fullChallengeTxs[0].PayloadHash())

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:         PeerMessageTypeSnapshotResponse,
		SnapshotHash: snap.PayloadHash(),
		Response:     response,
	})
	require.Nil(err)
	require.Equal(response, handle.snapshotResponse)

	err = me.handlePeerMessage(peerID, &PeerMessage{
		Type:     PeerMessageTypeSnapshotFinalization,
		Snapshot: snap,
	})
	require.Nil(err)
	require.Equal(snap.PayloadHash(), handle.finalization.PayloadHash())

	relayParsed, err := parseNetworkMessage(9, me.buildRelayMessage(me.IdForNetwork, buildTransactionRequestMessage(tx.PayloadHash())))
	require.Nil(err)
	err = me.handlePeerMessage(peerID, relayParsed)
	require.Nil(err)
	require.Equal(tx.PayloadHash(), handle.requestedTx)

	relayer := NewPeer(nil, peerID, "127.0.0.1:9012", true)
	me.relayers.Set(peerID, relayer)
	handle.authToken = &AuthToken{PeerId: crypto.Blake3Hash([]byte("consumer-id"))}
	consumerPeer := NewPeer(nil, handle.authToken.PeerId, "127.0.0.1:9013", false)
	consumerPeer.consumerAuth = &AuthToken{Data: bytes.Repeat([]byte{1}, 137)}
	msg, err := parseNetworkMessage(9, (&Peer{
		consumers: &neighborMap{m: map[crypto.Hash]*Peer{handle.authToken.PeerId: consumerPeer}},
	}).buildConsumersMessage())
	require.Nil(err)
	err = me.handlePeerMessage(peerID, msg)
	require.Nil(err)
	require.Len(me.GetRemoteRelayers(handle.authToken.PeerId), 1)
}

func TestCompareRoundGraphAndGetTopologicalOffset(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("self")), "127.0.0.1:9020", false)
	peer := NewPeer(nil, crypto.Blake3Hash([]byte("remote")), "127.0.0.1:9021", false)

	nodeA := crypto.Blake3Hash([]byte("node-a"))
	nodeB := crypto.Blake3Hash([]byte("node-b"))
	handle.roundSnapshots[roundKey(nodeA, 4)] = []*common.SnapshotWithTopologicalOrder{{
		Snapshot:         p2pTestSnapshot(false),
		TopologicalOrder: 9,
	}}
	handle.roundSnapshots[roundKey(nodeB, 3)] = []*common.SnapshotWithTopologicalOrder{{
		Snapshot:         p2pTestSnapshot(false),
		TopologicalOrder: 7,
	}}

	offset, err := me.compareRoundGraphAndGetTopologicalOffset(peer,
		[]*SyncPoint{
			{NodeId: nodeA, Number: 5},
			{NodeId: nodeB, Number: 3},
		},
		[]*SyncPoint{
			{NodeId: nodeA, Number: 2},
			{NodeId: nodeB, Number: 1},
		},
	)
	require.Nil(err)
	require.Equal(uint64(7), offset)

	handle.roundErr = errors.New("round read failed")
	_, err = me.compareRoundGraphAndGetTopologicalOffset(peer,
		[]*SyncPoint{{NodeId: nodeA, Number: 5}},
		[]*SyncPoint{{NodeId: nodeA, Number: 2}},
	)
	require.ErrorIs(err, handle.roundErr)
}

func TestSendMessageHelpers(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("sender")), "127.0.0.1:9030", true)
	me.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}

	target := crypto.Blake3Hash([]byte("target"))
	neighbor := NewPeer(nil, target, "127.0.0.1:9031", true)
	me.relayers.Set(target, neighbor)

	consumerID := crypto.Blake3Hash([]byte("consumer"))
	consumer := NewPeer(nil, consumerID, "127.0.0.1:9032", false)
	me.consumers.Set(consumerID, consumer)
	require.Len(me.Neighbors(), 2)

	snapshot := p2pTestSnapshot(true)
	snapshot.Hash = snapshot.PayloadHash()
	tx := p2pTestTransaction()
	fullTx := common.NewTransactionV5(common.XINAssetId)
	fullTx.Extra = bytes.Repeat([]byte{6}, 220)
	fullVer := fullTx.AsVersioned()
	commitment := p2pTestPrivateKey(41).Public()
	challenge := p2pTestPrivateKey(42).Public()
	spend := p2pTestPrivateKey(43)
	cosi := &crypto.CosiSignature{Mask: 9}
	response := &[32]byte{4, 5, 6}

	err := me.SendGraphMessage(target)
	require.Nil(err)
	require.EqualValues(PeerMessageTypeGraph, (<-neighbor.highRing).data[0])

	err = me.SendCommitmentsMessage(target, []*crypto.Key{&commitment})
	require.Nil(err)
	require.EqualValues(PeerMessageTypeCommitments, (<-neighbor.highRing).data[0])

	err = me.SendSnapshotAnnouncementMessage(target, snapshot, commitment, spend)
	require.Nil(err)
	require.EqualValues(PeerMessageTypeSnapshotAnnouncement, (<-neighbor.normalRing).data[0])

	err = me.SendSnapshotCommitmentMessage(target, snapshot.PayloadHash(), commitment, []crypto.Hash{tx.PayloadHash()})
	require.Nil(err)
	require.EqualValues(PeerMessageTypeSnapshotCommitment, (<-neighbor.normalRing).data[0])

	err = me.SendTransactionChallengeMessage(target, snapshot.PayloadHash(), cosi, []*common.VersionedTransaction{tx})
	require.Nil(err)
	require.EqualValues(PeerMessageTypeTransactionChallenge, (<-neighbor.normalRing).data[0])

	err = me.SendFullChallengeMessage(target, snapshot, &commitment, &challenge, []*common.VersionedTransaction{fullVer})
	require.Nil(err)
	require.EqualValues(PeerMessageTypeFullChallenge, (<-neighbor.normalRing).data[0])

	err = me.SendSnapshotResponseMessage(target, snapshot.PayloadHash(), response)
	require.Nil(err)
	require.EqualValues(PeerMessageTypeSnapshotResponse, (<-neighbor.normalRing).data[0])

	err = me.SendSnapshotConfirmMessage(target, snapshot.PayloadHash())
	require.Nil(err)
	require.EqualValues(PeerMessageTypeSnapshotConfirm, (<-neighbor.highRing).data[0])

	err = me.SendTransactionRequestMessage(target, tx.PayloadHash())
	require.Nil(err)
	require.EqualValues(PeerMessageTypeTransactionRequest, (<-neighbor.highRing).data[0])

	err = me.SendTransactionMessage(target, tx)
	require.Nil(err)
	require.EqualValues(PeerMessageTypeTransaction, (<-neighbor.highRing).data[0])

	err = me.SendSnapshotFinalizationMessage(me.IdForNetwork, snapshot)
	require.Nil(err)

	me.ConfirmSnapshotForPeer(target, snapshot.Hash)
	me.snapshotsCaches.cache.Wait()
	err = me.SendSnapshotFinalizationMessage(target, snapshot)
	require.Nil(err)
	require.Empty(neighbor.normalRing)

	relayID := crypto.Blake3Hash([]byte("relay"))
	relayer := NewPeer(nil, relayID, "127.0.0.1:9033", true)
	me.relayers.Set(relayID, relayer)
	remotePeer := crypto.Blake3Hash([]byte("remote-peer"))
	me.remoteRelayers.Add(remotePeer, relayID)
	err = me.SendTransactionRequestMessage(remotePeer, tx.PayloadHash())
	require.Nil(err)
	require.EqualValues(PeerMessageTypeRelay, (<-relayer.highRing).data[0])
}

func TestPeerLoopAndSyncHelpers(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("loop-self")), "127.0.0.1:9040", true)
	peer := NewPeer(nil, crypto.Blake3Hash([]byte("loop-peer")), "127.0.0.1:9041", true)

	cachedKey := []byte("cached")
	me.snapshotsCaches.store(cachedKey, time.Now())
	me.snapshotsCaches.cache.Wait()

	ring := make(chan *ChanMsg, 4)
	ring <- &ChanMsg{key: cachedKey, data: []byte("skip")}
	ring <- &ChanMsg{key: nil, data: []byte("keep")}
	msgs := me.pollRingWithCache(ring, 16)
	require.Len(msgs, 1)
	require.Equal([]byte("keep"), msgs[0].data)

	offerPeer := NewPeer(nil, crypto.Blake3Hash([]byte("offer")), "127.0.0.1:9042", false)
	offerPeer.normalRing = make(chan *ChanMsg)
	require.False(offerPeer.offer(MsgPriorityNormal, &ChanMsg{data: []byte("full")}))
	offerPeer.normalRing = make(chan *ChanMsg, 1)
	require.True(offerPeer.offer(MsgPriorityNormal, &ChanMsg{data: []byte("ok")}))
	offerPeer.highRing = make(chan *ChanMsg, 1)
	require.True(offerPeer.offer(MsgPriorityHigh, &ChanMsg{data: []byte("ok")}))
	offerPeer.closing = true
	require.False(offerPeer.offer(MsgPriorityHigh, &ChanMsg{data: []byte("closed")}))
	require.Panics(func() {
		offerPeer.closing = false
		offerPeer.offer(99, &ChanMsg{})
	})

	require.True(me.offerToPeerWithCacheCheck(me, MsgPriorityNormal, &ChanMsg{data: []byte("self")}))
	require.True(me.offerToPeerWithCacheCheck(peer, MsgPriorityNormal, &ChanMsg{key: cachedKey, data: []byte("cached")}))

	streamClient := &scriptedClient{addr: stubAddr("stream-client")}
	streamClient.sendHook = func([]byte) {
		if len(streamClient.sent) == 2 {
			me.closing = true
		}
	}
	peer.highRing <- &ChanMsg{key: []byte("stream-key"), data: []byte("high")}
	peer.normalRing <- &ChanMsg{data: []byte("normal")}
	msg, err := me.loopSendingStream(peer, streamClient)
	require.Nil(msg)
	require.ErrorContains(err, "PEER DONE")
	me.snapshotsCaches.cache.Wait()
	require.True(me.snapshotsCaches.contains([]byte("stream-key"), time.Hour))
	require.Len(streamClient.sent, 2)

	me2 := NewPeer(handle, crypto.Blake3Hash([]byte("loop-self-2")), "127.0.0.1:9043", true)
	peer2 := NewPeer(nil, crypto.Blake3Hash([]byte("loop-peer-2")), "127.0.0.1:9044", true)
	errorClient := &scriptedClient{addr: stubAddr("error-client"), sendErr: io.ErrClosedPipe, sendErrAt: 1}
	peer2.highRing <- &ChanMsg{data: []byte("boom")}
	msg, err = me2.loopSendingStream(peer2, errorClient)
	require.NotNil(msg)
	require.ErrorContains(err, "consumer.Send")

	hash := p2pTestTransaction().PayloadHash()
	receiveClient := &scriptedClient{
		addr: stubAddr("receive-client"),
		receiveSteps: []receiveStep{
			{msg: &TransportMessage{Version: TransportMessageVersion, Data: buildTransactionRequestMessage(hash)}},
			{err: io.EOF},
		},
	}
	me3 := NewPeer(handle, crypto.Blake3Hash([]byte("loop-self-3")), "127.0.0.1:9045", true)
	me3.receivedMetric.enabled = true
	peer3 := NewPeer(nil, crypto.Blake3Hash([]byte("loop-peer-3")), "127.0.0.1:9046", true)
	me3.loopReceiveMessage(peer3, receiveClient)
	require.Eventually(func() bool {
		return handle.requestedTx == hash
	}, time.Second, 10*time.Millisecond)
	require.Equal(uint32(1), me3.receivedMetric.PeerMessageTypeTransactionRequest)

	authClient := &scriptedClient{
		addr: stubAddr("auth-client"),
		receiveSteps: []receiveStep{
			{msg: &TransportMessage{Version: TransportMessageVersion, Data: buildAuthenticationMessage([]byte("auth"))}},
		},
	}
	me4 := NewPeer(handle, crypto.Blake3Hash([]byte("loop-self-4")), "127.0.0.1:9047", true)
	me4.receivedMetric.enabled = true
	authPeer, err := me4.authenticateNeighbor(authClient)
	require.Nil(err)
	require.Equal(handle.authToken.PeerId, authPeer.IdForNetwork)
	require.Equal(handle.authToken.IsRelayer, authPeer.IsRelayer())
	require.Equal(uint32(1), me4.receivedMetric.PeerMessageTypeAuthentication)

	badAuthClient := &scriptedClient{
		addr: stubAddr("bad-auth-client"),
		receiveSteps: []receiveStep{
			{msg: &TransportMessage{Version: TransportMessageVersion, Data: buildTransactionRequestMessage(hash)}},
		},
	}
	_, err = me4.authenticateNeighbor(badAuthClient)
	require.ErrorContains(err, "invalid message type")

	me5 := NewPeer(handle, crypto.Blake3Hash([]byte("sync-self")), "127.0.0.1:9048", true)
	peer5 := NewPeer(nil, crypto.Blake3Hash([]byte("sync-peer")), "127.0.0.1:9049", true)
	me5.relayers.Set(peer5.IdForNetwork, peer5)
	nodeID := crypto.Blake3Hash([]byte("sync-node"))
	s1 := p2pTestSnapshot(false)
	s1.NodeId = nodeID
	s1.RoundNumber = 0
	s1.Hash = s1.PayloadHash()
	s2 := p2pTestSnapshot(false)
	s2.NodeId = nodeID
	s2.RoundNumber = 2
	s2.Hash = s2.PayloadHash()
	handle.sinceSnapshots = []*common.SnapshotWithTopologicalOrder{
		{Snapshot: s1, TopologicalOrder: 5},
		{Snapshot: s2, TopologicalOrder: 6},
	}
	offset, err := me5.syncToNeighborSince(map[crypto.Hash]*SyncPoint{
		nodeID: {NodeId: nodeID, Number: 1},
	}, peer5, 1)
	require.ErrorContains(err, "EOF")
	require.Equal(uint64(6), offset)
	require.EqualValues(PeerMessageTypeSnapshotFinalization, (<-peer5.normalRing).data[0])

	handle.sinceSnapshots = []*common.SnapshotWithTopologicalOrder{{
		Snapshot: &common.Snapshot{
			Version:      common.SnapshotVersionCommonEncoding,
			NodeId:       nodeID,
			RoundNumber:  config.SnapshotReferenceThreshold * 2,
			Timestamp:    3,
			Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("future"))},
			Hash:         crypto.Blake3Hash([]byte("future-hash")),
		},
		TopologicalOrder: 7,
	}}
	_, err = me5.syncToNeighborSince(map[crypto.Hash]*SyncPoint{
		nodeID: {NodeId: nodeID, Number: 0},
	}, peer5, 1)
	require.ErrorContains(err, "FUTURE")

	handle.roundSnapshots = map[string][]*common.SnapshotWithTopologicalOrder{
		roundKey(nodeID, 0): {{Snapshot: s1, TopologicalOrder: 1}},
		roundKey(nodeID, 1): {{Snapshot: s2, TopologicalOrder: 2}},
		roundKey(nodeID, 5): {{Snapshot: s2, TopologicalOrder: 9}},
	}
	handle.graph = []*SyncPoint{{NodeId: nodeID, Number: 5}}
	me5.syncHeadRoundToRemote(
		map[crypto.Hash]*SyncPoint{nodeID: {NodeId: nodeID, Number: 2}},
		map[crypto.Hash]*SyncPoint{nodeID: {NodeId: nodeID, Number: 0}},
		peer5, nodeID,
	)
	require.EqualValues(PeerMessageTypeSnapshotFinalization, (<-peer5.normalRing).data[0])
	require.EqualValues(PeerMessageTypeSnapshotFinalization, (<-peer5.normalRing).data[0])

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				select {
				case peer5.syncRing <- []*SyncPoint{{NodeId: nodeID, Number: 3}}:
				default:
				}
			}
		}
	}()
	graph, offset := me5.getSyncPointOffset(peer5)
	close(done)
	require.NotNil(graph)
	require.Equal(uint64(9), offset)

	me6 := NewPeer(handle, crypto.Blake3Hash([]byte("relay-self")), "127.0.0.1:9050", true)
	nextRelay := NewPeer(nil, crypto.Blake3Hash([]byte("relay-next")), "127.0.0.1:9051", true)
	me6.relayers.Set(nextRelay.IdForNetwork, nextRelay)
	err = me6.relayOrHandlePeerMessage(nextRelay.IdForNetwork, &PeerMessage{Data: []byte("short"), version: TransportMessageVersion})
	require.Nil(err)

	target := crypto.Blake3Hash([]byte("relay-target"))
	relayData := me6.buildRelayMessage(target, buildTransactionRequestMessage(hash))
	err = me6.relayOrHandlePeerMessage(crypto.Blake3Hash([]byte("relay-origin")), &PeerMessage{Data: relayData, version: TransportMessageVersion})
	require.Nil(err)
	select {
	case forwarded := <-nextRelay.normalRing:
		require.EqualValues(PeerMessageTypeRelay, forwarded.data[0])
	case forwarded := <-nextRelay.highRing:
		require.EqualValues(PeerMessageTypeRelay, forwarded.data[0])
	default:
	}

	me7 := NewPeer(nil, crypto.Blake3Hash([]byte("non-relayer")), "127.0.0.1:9052", false)
	err = me7.relayOrHandlePeerMessage(crypto.Blake3Hash([]byte("origin")), &PeerMessage{Data: relayData, version: TransportMessageVersion})
	require.Nil(err)

	tear := NewPeer(handle, crypto.Blake3Hash([]byte("tear")), "127.0.0.1:9053", true)
	child1 := NewPeer(nil, crypto.Blake3Hash([]byte("tear-1")), "127.0.0.1:9054", false)
	child2 := NewPeer(nil, crypto.Blake3Hash([]byte("tear-2")), "127.0.0.1:9055", false)
	close(child1.ops)
	close(child1.stn)
	close(child2.ops)
	close(child2.stn)
	tear.relayers.Set(child1.IdForNetwork, child1)
	tear.consumers.Set(child2.IdForNetwork, child2)
	tear.Teardown()
	require.True(tear.closing)
	require.True(child1.closing)
	require.True(child2.closing)
}

func TestConnectRelayerAndListenConsumersIntegration(t *testing.T) {
	require := require.New(t)

	serverHandle := newP2PStubHandle(t)
	clientHandle := newP2PStubHandle(t)
	serverID := crypto.Blake3Hash([]byte("integration-server"))
	clientID := crypto.Blake3Hash([]byte("integration-client"))
	serverHandle.authToken = &AuthToken{
		PeerId:    clientID,
		IsRelayer: true,
		Data:      bytes.Repeat([]byte{8}, 137),
	}

	server := NewPeer(serverHandle, serverID, "127.0.0.1:0", true)
	listenDone := make(chan error, 1)
	go func() {
		listenDone <- server.ListenConsumers()
	}()
	require.Eventually(func() bool {
		return server.relayer != nil
	}, time.Second, 10*time.Millisecond)
	addr := server.relayer.listener.Addr().String()

	client := NewPeer(clientHandle, clientID, "127.0.0.1:0", true)
	client.sentMetric.enabled = true
	relay := NewPeer(nil, serverID, addr, true)
	connectDone := make(chan error, 1)
	go func() {
		connectDone <- client.connectRelayer(relay)
	}()

	require.Eventually(func() bool {
		return client.relayers.Get(serverID) != nil && server.consumers.Get(clientID) != nil
	}, 3*time.Second, 20*time.Millisecond)
	require.Equal(uint32(1), client.sentMetric.PeerMessageTypeAuthentication)

	client.closing = true
	relay.closing = true
	require.Eventually(func() bool {
		select {
		case <-connectDone:
			return true
		default:
			return false
		}
	}, 3*time.Second, 20*time.Millisecond)

	server.closing = true
	require.NoError(server.relayer.Close())
	require.Eventually(func() bool {
		select {
		case err := <-listenDone:
			require.Nil(err)
			return true
		default:
			return false
		}
	}, 3*time.Second, 20*time.Millisecond)

	top := NewPeer(newP2PStubHandle(t), crypto.Blake3Hash([]byte("top-connect")), "127.0.0.1:0", true)
	top.closing = true
	top.ConnectRelayer(serverID, addr)
	require.NotNil(top.remoteRelayers)

	require.Panics(func() {
		NewPeer(nil, crypto.Blake3Hash([]byte("bad-connect")), "127.0.0.1:0", false).ConnectRelayer(serverID, "not-an-addr")
	})
	require.Panics(func() {
		NewPeer(nil, crypto.Blake3Hash([]byte("small-port")), "127.0.0.1:0", false).ConnectRelayer(serverID, "127.0.0.1:1")
	})
}

func TestSyncToNeighborLoop(t *testing.T) {
	require := require.New(t)

	handle := newP2PStubHandle(t)
	nodeID := crypto.Blake3Hash([]byte("sync-loop-node"))
	s1 := p2pTestSnapshot(false)
	s1.NodeId = nodeID
	s1.RoundNumber = 0
	s1.Hash = s1.PayloadHash()
	handle.graph = []*SyncPoint{{NodeId: nodeID, Number: 3, Hash: crypto.Blake3Hash([]byte("graph"))}}
	handle.nodes = []crypto.Hash{nodeID}
	handle.roundSnapshots = map[string][]*common.SnapshotWithTopologicalOrder{
		roundKey(nodeID, 2): {{Snapshot: s1, TopologicalOrder: 3}},
	}
	handle.sinceSnapshots = []*common.SnapshotWithTopologicalOrder{{
		Snapshot:         s1,
		TopologicalOrder: 4,
	}}

	me := NewPeer(handle, crypto.Blake3Hash([]byte("sync-loop-self")), "127.0.0.1:0", true)
	peer := NewPeer(nil, crypto.Blake3Hash([]byte("sync-loop-peer")), "127.0.0.1:0", true)
	me.relayers.Set(peer.IdForNetwork, peer)

	done := make(chan struct{})
	go func() {
		me.syncToNeighborLoop(peer)
		close(done)
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	require.Eventually(func() bool {
		select {
		case <-ticker.C:
			select {
			case peer.syncRing <- []*SyncPoint{{NodeId: nodeID, Number: 0}}:
			default:
			}
		default:
		}
		return len(peer.normalRing) > 0
	}, 3*time.Second, 20*time.Millisecond)

	me.closing = true
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("syncToNeighborLoop did not exit")
	}
}

type p2pStubHandle struct {
	cache *ristretto.Cache[[]byte, any]
	key   crypto.Key

	authToken *AuthToken
	graph     []*SyncPoint
	nodes     []crypto.Hash
	authErr   error

	lastCommitments  []*crypto.Key
	updatePoints     []*SyncPoint
	updateData       []byte
	updateSig        *crypto.Signature
	updateErr        error
	requestedTx      crypto.Hash
	cachedTx         *common.VersionedTransaction
	announcement     *common.Snapshot
	wantTxs          []crypto.Hash
	challengeTxs     []*common.VersionedTransaction
	fullChallengeTxs []*common.VersionedTransaction
	snapshotResponse [32]byte
	finalization     *common.Snapshot

	roundSnapshots map[string][]*common.SnapshotWithTopologicalOrder
	roundErr       error
	sinceSnapshots []*common.SnapshotWithTopologicalOrder
	sinceErr       error
}

func newP2PStubHandle(t *testing.T) *p2pStubHandle {
	t.Helper()

	cache := newP2PTestCache(t)
	key := p2pTestPrivateKey(91)
	return &p2pStubHandle{
		cache: cache,
		key:   key,
		authToken: &AuthToken{
			PeerId:    crypto.Blake3Hash([]byte("auth-peer")),
			Timestamp: 1,
		},
		graph: []*SyncPoint{{
			NodeId: crypto.Blake3Hash([]byte("graph-node")),
			Number: 3,
			Hash:   crypto.Blake3Hash([]byte("graph-hash")),
		}},
		roundSnapshots: make(map[string][]*common.SnapshotWithTopologicalOrder),
	}
}

func (h *p2pStubHandle) GetCacheStore() *ristretto.Cache[[]byte, any] {
	return h.cache
}

func (h *p2pStubHandle) SignData(data []byte) crypto.Signature {
	return h.key.Sign(crypto.Blake3Hash(data))
}

func (h *p2pStubHandle) BuildAuthenticationMessage(relayerId crypto.Hash) []byte {
	return append(relayerId[:], byte(1))
}

func (h *p2pStubHandle) AuthenticateAs(_ crypto.Hash, _ []byte, _ int64) (*AuthToken, error) {
	if h.authErr != nil {
		return nil, h.authErr
	}
	return h.authToken, nil
}

func (h *p2pStubHandle) BuildGraph() []*SyncPoint {
	return h.graph
}

func (h *p2pStubHandle) UpdateSyncPoint(_ crypto.Hash, points []*SyncPoint, data []byte, sig *crypto.Signature) error {
	h.updatePoints = points
	h.updateData = data
	h.updateSig = sig
	return h.updateErr
}

func (h *p2pStubHandle) ReadAllNodesWithoutState() []crypto.Hash {
	return h.nodes
}

func (h *p2pStubHandle) ReadSnapshotsSinceTopology(_, _ uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	if h.sinceErr != nil {
		return nil, h.sinceErr
	}
	return h.sinceSnapshots, nil
}

func (h *p2pStubHandle) ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	if h.roundErr != nil {
		return nil, h.roundErr
	}
	return h.roundSnapshots[roundKey(nodeIdWithNetwork, round)], nil
}

func (h *p2pStubHandle) SendTransactionToPeer(_ crypto.Hash, tx crypto.Hash) error {
	h.requestedTx = tx
	return nil
}

func (h *p2pStubHandle) CachePutTransaction(_ crypto.Hash, ver *common.VersionedTransaction) error {
	h.cachedTx = ver
	return nil
}

func (h *p2pStubHandle) CosiQueueExternalAnnouncement(_ crypto.Hash, s *common.Snapshot, _ *crypto.Key, _ *crypto.Signature) error {
	h.announcement = s
	return nil
}

func (h *p2pStubHandle) CosiAggregateSelfCommitments(_ crypto.Hash, _ crypto.Hash, _ *crypto.Key, wantTxs []crypto.Hash, _ []byte, _ *crypto.Signature) error {
	h.wantTxs = wantTxs
	return nil
}

func (h *p2pStubHandle) CosiQueueExternalChallenge(_ crypto.Hash, _ crypto.Hash, _ *crypto.CosiSignature, txs []*common.VersionedTransaction) error {
	h.challengeTxs = txs
	return nil
}

func (h *p2pStubHandle) CosiQueueExternalFullChallenge(_ crypto.Hash, _ *common.Snapshot, _ *crypto.Key, _ *crypto.Key, _ *crypto.CosiSignature, txs []*common.VersionedTransaction) error {
	h.fullChallengeTxs = txs
	return nil
}

func (h *p2pStubHandle) CosiAggregateSelfResponses(_ crypto.Hash, _ crypto.Hash, response *[32]byte) error {
	h.snapshotResponse = *response
	return nil
}

func (h *p2pStubHandle) VerifyAndQueueAppendSnapshotFinalization(_ crypto.Hash, s *common.Snapshot) error {
	h.finalization = s
	return nil
}

func (h *p2pStubHandle) CosiQueueExternalCommitments(_ crypto.Hash, commitments []*crypto.Key, _ []byte, _ *crypto.Signature) error {
	h.lastCommitments = commitments
	return nil
}

func newP2PTestCache(t *testing.T) *ristretto.Cache[[]byte, any] {
	t.Helper()

	cache, err := ristretto.NewCache(&ristretto.Config[[]byte, any]{
		NumCounters: 1e4,
		MaxCost:     1 << 20,
		BufferItems: 64,
	})
	require.NoError(t, err)
	return cache
}

func p2pTestPrivateKey(seed byte) crypto.Key {
	src := bytes.Repeat([]byte{seed}, 64)
	return crypto.NewKeyFromSeed(src)
}

func p2pTestTransaction() *common.VersionedTransaction {
	return common.NewTransactionV5(common.XINAssetId).AsVersioned()
}

func p2pTestSnapshot(withSignature bool) *common.Snapshot {
	s := &common.Snapshot{
		Version:     common.SnapshotVersionCommonEncoding,
		NodeId:      crypto.Blake3Hash([]byte("snapshot-node")),
		RoundNumber: 7,
		References: &common.RoundLink{
			Self:     crypto.Blake3Hash([]byte("self")),
			External: crypto.Blake3Hash([]byte("external")),
		},
		Timestamp:    11,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("snapshot-transaction"))},
	}
	if withSignature {
		s.Signature = &crypto.CosiSignature{Mask: 1}
	}
	return s
}

func roundKey(node crypto.Hash, number uint64) string {
	return fmt.Sprintf("%s:%d", node.String(), number)
}

type receiveStep struct {
	msg *TransportMessage
	err error
}

type scriptedClient struct {
	addr         net.Addr
	receiveSteps []receiveStep
	sendErr      error
	sendErrAt    int
	sendHook     func([]byte)

	mu         sync.Mutex
	sendCount  int
	sent       [][]byte
	closeCodes []string
}

func (c *scriptedClient) RemoteAddr() net.Addr {
	if c.addr != nil {
		return c.addr
	}
	return stubAddr("scripted")
}

func (c *scriptedClient) Receive() (*TransportMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.receiveSteps) == 0 {
		return nil, io.EOF
	}
	step := c.receiveSteps[0]
	c.receiveSteps = c.receiveSteps[1:]
	return step.msg, step.err
}

func (c *scriptedClient) Send(data []byte) error {
	c.mu.Lock()
	c.sendCount += 1
	c.sent = append(c.sent, append([]byte{}, data...))
	count := c.sendCount
	hook := c.sendHook
	errAt := c.sendErrAt
	errVal := c.sendErr
	c.mu.Unlock()

	if hook != nil {
		hook(data)
	}
	if errAt > 0 && count == errAt {
		return errVal
	}
	return nil
}

func (c *scriptedClient) Close(code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeCodes = append(c.closeCodes, code)
	return nil
}

type stubAddr string

func (a stubAddr) Network() string { return "udp" }

func (a stubAddr) String() string { return string(a) }
