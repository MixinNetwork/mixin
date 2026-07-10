package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestProtocolPayloadValidationBranches(t *testing.T) {
	tx := p2pTestTransaction()
	snapshot := p2pTestSnapshot(true)
	commitment := p2pTestPrivateKey(201).Public()
	challenge := p2pTestPrivateKey(202).Public()

	require.Nil(t, soleTransaction(nil))
	require.Panics(t, func() {
		soleTransaction([]*common.VersionedTransaction{tx, tx})
	})
	withoutHash := p2pTestSnapshot(false)
	withoutHash.Hash = crypto.Hash{}
	require.Equal(t, withoutHash.PayloadHash(), snapshotHash(withoutHash))
	require.Panics(t, func() {
		buildTransactionsPayload(make([]*common.VersionedTransaction, common.SnapshotTransactionsMaximum+1))
	})

	validPayload := buildTransactionsPayload([]*common.VersionedTransaction{tx})
	transactionPayloads := [][]byte{
		nil,
		{1},
		{1, 0, 0, 0, 2, 0},
		{1, 0, 0, 0, 1, 0},
		append(append([]byte(nil), validPayload...), 0xff),
	}
	for _, payload := range transactionPayloads {
		_, err := parseTransactionsPayload(payload)
		require.Error(t, err)
	}

	malformedBatchCommitment := make([]byte, 130)
	malformedBatchCommitment[0] = PeerMessageTypeBatchSnapshotCommitment
	malformedTransactionChallenge := buildTransactionChallengeMessage(
		snapshot.PayloadHash(),
		&crypto.CosiSignature{Mask: 1},
		nil,
	)
	malformedTransactionChallenge = append(malformedTransactionChallenge, 0)
	malformedBatchChallenge := buildBatchTransactionChallengeMessage(
		snapshot.PayloadHash(),
		&crypto.CosiSignature{Mask: 1},
		nil,
	)
	malformedBatchChallenge[len(malformedBatchChallenge)-1] = 1

	messages := [][]byte{
		{PeerMessageTypePreCommitments},
		{PeerMessageTypeTransaction, 0},
		{PeerMessageTypeTransactionBundle},
		{PeerMessageTypeSnapshotCommitment},
		{PeerMessageTypeBatchSnapshotCommitment},
		malformedBatchCommitment,
		malformedTransactionChallenge,
		{PeerMessageTypeBatchTransactionChallenge},
		malformedBatchChallenge,
		{PeerMessageTypeSnapshotFinalization, 0},
	}
	for _, message := range messages {
		_, err := parseNetworkMessage(7, message)
		require.Error(t, err)
	}

	full := buildFullChallengeMessage(snapshot, &commitment, &challenge, tx)
	fullSnapshotSize := int(binary.BigEndian.Uint32(full[1:5]))
	fullAfterSnapshot := 5 + fullSnapshotSize

	badFullSnapshot := append([]byte(nil), full...)
	binary.BigEndian.PutUint32(badFullSnapshot[1:5], 1)
	badFullSnapshot[5] = 0
	_, err := parseNetworkMessage(7, badFullSnapshot)
	require.ErrorContains(t, err, "invalid full challenge snapshot")

	badFullSnapshotSize := append([]byte(nil), full...)
	binary.BigEndian.PutUint32(badFullSnapshotSize[1:5], uint32(len(full)))
	_, err = parseNetworkMessage(7, badFullSnapshotSize)
	require.ErrorContains(t, err, "invalid full challenge snapshot size")

	unsignedFull := buildFullChallengeMessage(p2pTestSnapshot(false), &commitment, &challenge, tx)
	_, err = parseNetworkMessage(7, unsignedFull)
	require.ErrorContains(t, err, "snapshot signature")

	shortFullTail := append([]byte(nil), full[:fullAfterSnapshot+67]...)
	_, err = parseNetworkMessage(7, shortFullTail)
	require.ErrorContains(t, err, "invalid full challenge message size")

	badFullTransactionSize := append([]byte(nil), full...)
	transactionSizeOffset := fullAfterSnapshot + 64
	binary.BigEndian.PutUint32(badFullTransactionSize[transactionSizeOffset:transactionSizeOffset+4], 1<<20)
	_, err = parseNetworkMessage(7, badFullTransactionSize)
	require.ErrorContains(t, err, "transaction size")

	badFullTransaction := append([]byte(nil), full[:transactionSizeOffset+5]...)
	binary.BigEndian.PutUint32(badFullTransaction[transactionSizeOffset:transactionSizeOffset+4], 1)
	badFullTransaction[transactionSizeOffset+4] = 0
	_, err = parseNetworkMessage(7, badFullTransaction)
	require.ErrorContains(t, err, "invalid full challenge transaction")

	batch := buildBatchFullChallengeMessage(snapshot, &commitment, &challenge, []*common.VersionedTransaction{tx})
	batchSnapshotSize := int(binary.BigEndian.Uint32(batch[1:5]))
	batchAfterSnapshot := 5 + batchSnapshotSize

	_, err = parseNetworkMessage(7, []byte{PeerMessageTypeBatchFullChallenge})
	require.ErrorContains(t, err, "invalid full challenge message size")

	badBatchSnapshot := append([]byte(nil), batch...)
	binary.BigEndian.PutUint32(badBatchSnapshot[1:5], 1)
	badBatchSnapshot[5] = 0
	_, err = parseNetworkMessage(7, badBatchSnapshot)
	require.ErrorContains(t, err, "invalid full challenge snapshot")

	unsignedBatch := buildBatchFullChallengeMessage(p2pTestSnapshot(false), &commitment, &challenge, []*common.VersionedTransaction{tx})
	_, err = parseNetworkMessage(7, unsignedBatch)
	require.ErrorContains(t, err, "snapshot signature")

	shortBatchTail := append([]byte(nil), batch[:batchAfterSnapshot+64]...)
	_, err = parseNetworkMessage(7, shortBatchTail)
	require.ErrorContains(t, err, "invalid full challenge message size")

	badBatchTransactions := append([]byte(nil), batch[:batchAfterSnapshot+64]...)
	badBatchTransactions = append(badBatchTransactions, 1)
	_, err = parseNetworkMessage(7, badBatchTransactions)
	require.ErrorContains(t, err, "invalid transactions payload")

	encodedPoints := marshalSyncPoints([]*SyncPoint{{
		NodeId: crypto.Blake3Hash([]byte("truncated node")),
		Number: 3,
		Hash:   crypto.Blake3Hash([]byte("truncated hash")),
	}})
	for i := range encodedPoints {
		_, err := unmarshalSyncPoints(encodedPoints[:i])
		require.Error(t, err)
	}
}

func TestPeerRoutingAndSynchronizationBranches(t *testing.T) {
	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("routing self")), "127.0.0.1:0", true)
	target := crypto.Blake3Hash([]byte("routing target"))

	require.NoError(t, me.SendTransactionsMessage(target, nil))
	require.Panics(t, func() {
		me.ConnectRelayer(target, "127.0.0.1:79")
	})

	expired := crypto.Blake3Hash([]byte("expired relayer"))
	me.remoteRelayers = &relayersMap{m: map[crypto.Hash][]*remoteRelayer{
		target: {{Id: expired, ActiveAt: time.Now().Add(-2 * time.Minute)}},
	}}
	require.Empty(t, me.remoteRelayers.Get(target))

	local := []*SyncPoint{{NodeId: target, Number: 1}}
	remote := []*SyncPoint{{NodeId: target, Number: 2}}
	offset, err := me.compareRoundGraphAndGetTopologicalOffset(NewPeer(nil, target, "remote", false), local, remote)
	require.NoError(t, err)
	require.Zero(t, offset)
	remote[0].Number = 1
	offset, err = me.compareRoundGraphAndGetTopologicalOffset(NewPeer(nil, target, "remote", false), local, remote)
	require.NoError(t, err)
	require.Zero(t, offset)

	handle.sinceErr = errors.New("snapshot read failed")
	offset, err = me.syncToNeighborSince(nil, NewPeer(nil, target, "remote", false), 9)
	require.Equal(t, uint64(9), offset)
	require.ErrorIs(t, err, handle.sinceErr)
	handle.sinceErr = nil

	handle.sinceSnapshots = make([]*common.SnapshotWithTopologicalOrder, 200)
	for i := range handle.sinceSnapshots {
		snapshot := p2pTestSnapshot(true)
		handle.sinceSnapshots[i] = &common.SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: uint64(i + 1),
		}
	}
	offset, err = me.syncToNeighborSince(nil, NewPeer(nil, target, "remote", false), 1)
	require.NoError(t, err)
	require.Equal(t, uint64(200), offset)

	me.syncHeadRoundToRemote(
		map[crypto.Hash]*SyncPoint{target: {NodeId: target, Number: 1}},
		map[crypto.Hash]*SyncPoint{target: {NodeId: target, Number: 2}},
		NewPeer(nil, target, "remote", false),
		target,
	)

	consumer := NewPeer(nil, target, "consumer", false)
	me.consumers.Set(target, consumer)
	require.Equal(t, []*Peer{consumer}, me.GetNeighbors(target))
	me.consumers.Delete(target)

	full := NewPeer(handle, crypto.Blake3Hash([]byte("full rings")), "full", true)
	for range cap(full.normalRing) {
		full.normalRing <- &ChanMsg{data: []byte("normal")}
	}
	for range cap(full.highRing) {
		full.highRing <- &ChanMsg{data: []byte("high")}
	}
	require.False(t, full.offer(MsgPriorityNormal, &ChanMsg{}))
	require.False(t, full.offer(MsgPriorityHigh, &ChanMsg{}))

	me.relayers.Set(target, full)
	require.NoError(t, me.sendToPeer(target, PeerMessageTypePing, nil, []byte("full"), MsgPriorityNormal))
	me.relayers.Delete(target)

	nonRelayerID := crypto.Blake3Hash([]byte("not a relayer"))
	me.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	me.remoteRelayers.Add(target, nonRelayerID)
	me.relayers.Set(nonRelayerID, NewPeer(nil, nonRelayerID, "not-relayer", false))
	require.Panics(t, func() {
		_ = me.sendToPeer(target, PeerMessageTypePing, nil, []byte("relay"), MsgPriorityNormal)
	})
	me.relayers.Delete(nonRelayerID)

	innerRelay := append([]byte{PeerMessageTypeRelay}, bytes.Repeat([]byte{0}, 64)...)
	copy(innerRelay[1:33], target[:])
	copy(innerRelay[33:65], me.IdForNetwork[:])
	err = me.relayOrHandlePeerMessage(target, &PeerMessage{version: 7, Data: innerRelay})
	require.Error(t, err)

	forward := NewPeer(handle, crypto.Blake3Hash([]byte("forward self")), "forward", true)
	forward.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	forwardPeer := NewPeer(nil, target, "forward target", true)
	forward.relayers.Set(target, forwardPeer)
	relay := forward.buildRelayMessage(target, []byte{PeerMessageTypePing})
	require.NoError(t, forward.relayOrHandlePeerMessage(crypto.Blake3Hash([]byte("source")), &PeerMessage{Data: relay}))
	select {
	case <-forwardPeer.normalRing:
	default:
		t.Fatal("forwarded relay was not queued")
	}
	require.NoError(t, forward.relayOrHandlePeerMessage(target, &PeerMessage{Data: relay}))
	for range cap(forwardPeer.normalRing) {
		forwardPeer.normalRing <- &ChanMsg{data: []byte("full relay")}
	}
	require.NoError(t, forward.relayOrHandlePeerMessage(crypto.Blake3Hash([]byte("other source")), &PeerMessage{Data: relay}))

}

func TestPeerReceiveAndAuthenticationFailures(t *testing.T) {
	handle := newP2PStubHandle(t)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("receive self")), "receive", true)
	peer := NewPeer(nil, crypto.Blake3Hash([]byte("receive peer")), "remote", false)

	handle.cacheErr = errors.New("cache rejected transaction")
	receiveClient := &scriptedClient{receiveSteps: []receiveStep{
		{msg: &TransportMessage{Version: 7, Data: buildTransactionMessage(p2pTestTransaction())}},
		{err: io.EOF},
	}}
	me.loopReceiveMessage(peer, receiveClient)
	require.Eventually(t, func() bool {
		receiveClient.mu.Lock()
		defer receiveClient.mu.Unlock()
		for _, code := range receiveClient.closeCodes {
			if code == "handlePeerMessage" {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
	handle.cacheErr = nil

	malformedClient := &scriptedClient{receiveSteps: []receiveStep{
		{msg: &TransportMessage{Version: 7}},
	}}
	me.loopReceiveMessage(peer, malformedClient)

	_, err := me.authenticateNeighbor(&scriptedClient{receiveSteps: []receiveStep{{err: io.EOF}}})
	require.ErrorIs(t, err, io.EOF)
	_, err = me.authenticateNeighbor(&scriptedClient{receiveSteps: []receiveStep{{
		msg: &TransportMessage{Version: 7},
	}}})
	require.Error(t, err)

	handle.authErr = errors.New("authentication rejected")
	_, err = me.authenticateNeighbor(&scriptedClient{receiveSteps: []receiveStep{{
		msg: &TransportMessage{Version: 7, Data: buildAuthenticationMessage([]byte("auth"))},
	}}})
	require.ErrorIs(t, err, handle.authErr)

}
