package p2p

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestConsensusMessageSignatureDispatch(t *testing.T) {
	sender := crypto.Blake3Hash([]byte("consensus sender"))
	receiver := crypto.Blake3Hash([]byte("consensus receiver"))
	relayer := crypto.Blake3Hash([]byte("consensus relayer"))
	unknown := crypto.Blake3Hash([]byte("unknown consensus sender"))
	snapshot := p2pTestSnapshot(true)
	transaction := p2pTestTransaction()
	commitment := p2pTestPrivateKey(61).Public()
	challenge := p2pTestPrivateKey(62).Public()
	replacement := p2pTestPrivateKey(63).Public()
	attacker := p2pTestPrivateKey(64)

	for _, message := range []struct {
		name             string
		build            func(*p2pStubHandle) []byte
		commitmentOffset int
		dispatched       func(*p2pStubHandle) bool
	}{
		{
			name: "precommitments",
			build: func(h *p2pStubHandle) []byte {
				return buildCommitmentsMessage(h, []*crypto.Key{&commitment})
			},
			commitmentOffset: 67,
			dispatched:       func(h *p2pStubHandle) bool { return len(h.lastCommitments) != 0 },
		},
		{
			name: "announcement",
			build: func(h *p2pStubHandle) []byte {
				return buildBatchSnapshotAnnouncementMessage(snapshot, commitment, h.key)
			},
			commitmentOffset: 65,
			dispatched:       func(h *p2pStubHandle) bool { return h.announcement != nil },
		},
		{
			name: "snapshot commitment",
			build: func(h *p2pStubHandle) []byte {
				return buildBatchSnapshotCommitmentMessage(h, snapshot.PayloadHash(), commitment, []crypto.Hash{transaction.PayloadHash()})
			},
			commitmentOffset: 97,
			dispatched:       func(h *p2pStubHandle) bool { return len(h.wantTxs) != 0 },
		},
		{
			name: "full challenge",
			build: func(h *p2pStubHandle) []byte {
				return buildBatchFullChallengeMessage(h, snapshot, &commitment, &challenge, []*common.VersionedTransaction{transaction})
			},
			commitmentOffset: 69 + len(snapshot.VersionedMarshal()),
			dispatched:       func(h *p2pStubHandle) bool { return h.fullChallenge != nil },
		},
	} {
		for _, route := range []string{"direct", "relayed"} {
			for _, variant := range []string{"valid", "invalid signature", "wrong signing key", "unknown peer", "changed commitment"} {
				t.Run(message.name+"/"+route+"/"+variant, func(t *testing.T) {
					handle := newP2PStubHandle(t)
					t.Cleanup(handle.cache.Close)
					handle.consensusPeers = map[crypto.Hash]crypto.Key{sender: handle.key.Public()}
					me := NewPeer(handle, receiver, "test", false)
					wire := message.build(handle)
					original, err := parseNetworkMessage(TransportMessageVersion, wire)
					require.NoError(t, err)
					require.Equal(t, wire[65:], original.unsigned)
					require.True(t, handle.VerifyConsensusPeerSignature(sender, wire[65:], original.signature))
					claimedSender := sender
					switch variant {
					case "invalid signature":
						wire[1] ^= 1
					case "wrong signing key":
						sig := attacker.Sign(crypto.Blake3Hash(original.unsigned))
						copy(wire[1:65], sig[:])
					case "unknown peer":
						claimedSender = unknown
					case "changed commitment":
						copy(wire[message.commitmentOffset:message.commitmentOffset+32], replacement[:])
					}

					peerID := claimedSender
					if route == "relayed" {
						origin := &Peer{IdForNetwork: claimedSender}
						wire = origin.buildRelayMessage(receiver, wire)
						peerID = relayer
					}
					parsed, err := parseNetworkMessage(TransportMessageVersion, wire)
					require.NoError(t, err)
					require.NoError(t, me.handlePeerMessage(peerID, parsed))
					require.Equal(t, variant == "valid", message.dispatched(handle))
				})
			}
		}
	}
}

func TestFullChallengeSignatureWireFormat(t *testing.T) {
	handle := newP2PStubHandle(t)
	t.Cleanup(handle.cache.Close)
	sender := crypto.Blake3Hash([]byte("full challenge sender"))
	handle.consensusPeers = map[crypto.Hash]crypto.Key{sender: handle.key.Public()}
	me := NewPeer(handle, crypto.Blake3Hash([]byte("full challenge receiver")), "test", false)
	snapshot := p2pTestSnapshot(true)
	commitment := p2pTestPrivateKey(71).Public()
	challenge := p2pTestPrivateKey(72).Public()
	transaction := p2pTestTransaction()
	wire := buildBatchFullChallengeMessage(handle, snapshot, &commitment, &challenge, []*common.VersionedTransaction{transaction})

	// All signed consensus messages authenticate the payload after the header.
	unsigned := wire[65:]
	sig := handle.SignData(unsigned)
	require.Equal(t, sig[:], wire[1:65])
	require.EqualValues(t, len(snapshot.VersionedMarshal()), binary.BigEndian.Uint32(wire[65:69]))
	parsed, err := parseNetworkMessage(TransportMessageVersion, wire)
	require.NoError(t, err)
	require.Equal(t, unsigned, parsed.unsigned)
	require.Equal(t, &sig, parsed.signature)
	require.NoError(t, me.handlePeerMessage(sender, parsed))
	require.NotNil(t, handle.fullChallenge)
	require.Equal(t, snapshot.PayloadHash(), handle.fullChallenge.PayloadHash())

	unsignedMessage := append([]byte{PeerMessageTypeBatchFullChallenge}, unsigned...)
	typeSignature := handle.SignData(unsignedMessage)
	typeSignedMessage := bytes.Clone(wire)
	copy(typeSignedMessage[1:65], typeSignature[:])

	// Missing signatures and the former signing conventions must never deliver
	// a challenge to the consensus queue.
	for _, malformed := range [][]byte{
		unsignedMessage,
		append(bytes.Clone(unsignedMessage), typeSignature[:]...),
		typeSignedMessage,
	} {
		handle.fullChallenge = nil
		msg, err := parseNetworkMessage(TransportMessageVersion, malformed)
		if err == nil {
			require.NoError(t, me.handlePeerMessage(sender, msg))
		}
		require.Nil(t, handle.fullChallenge)
	}

	// Exercise every signature and payload bit. Invalid encodings may be
	// rejected by the parser; otherwise authentication must reject the change.
	handle.fullChallenge = nil
	for offset := 1; offset < len(wire); offset++ {
		for bit := range 8 {
			mutated := bytes.Clone(wire)
			mutated[offset] ^= 1 << bit
			msg, err := parseNetworkMessage(TransportMessageVersion, mutated)
			if err != nil {
				continue
			}
			require.NoError(t, me.handlePeerMessage(sender, msg))
			require.Nil(t, handle.fullChallenge, "tampered byte %d bit %d reached consensus", offset, bit)
		}
	}
}

func TestFullChallengeRejectsOtherSignedPayloads(t *testing.T) {
	handle := newP2PStubHandle(t)
	t.Cleanup(handle.cache.Close)
	sender := crypto.Blake3Hash([]byte("retyped message sender"))
	snapshot := p2pTestSnapshot(false)
	snapshot.Transactions = nil
	handle.graph = nil
	var commitments []*crypto.Key
	for i := byte(1); i <= 10; i++ {
		commitment := p2pTestPrivateKey(i).Public()
		commitments = append(commitments, &commitment)
		hash := crypto.Blake3Hash([]byte{i})
		snapshot.AddTransaction(hash)
		handle.graph = append(handle.graph, &SyncPoint{NodeId: hash, Number: 1, Hash: hash})
	}

	for _, test := range []struct {
		name string
		wire []byte
	}{
		{name: "graph", wire: buildGraphMessage(handle)},
		{name: "precommitments", wire: buildCommitmentsMessage(handle, commitments)},
		{name: "announcement", wire: buildBatchSnapshotAnnouncementMessage(snapshot, *commitments[0], handle.key)},
		{name: "snapshot commitment", wire: buildBatchSnapshotCommitmentMessage(handle, snapshot.PayloadHash(), *commitments[0], snapshot.Transactions)},
	} {
		t.Run(test.name, func(t *testing.T) {
			original, err := parseNetworkMessage(TransportMessageVersion, test.wire)
			require.NoError(t, err)
			require.True(t, handle.VerifyConsensusPeerSignature(sender, original.unsigned, original.signature))
			// Keep the payload and signature intact, and ensure these fixtures
			// exercise format validation beyond the minimum frame length.
			require.GreaterOrEqual(t, len(test.wire), 1+64+256)
			retyped := bytes.Clone(test.wire)
			retyped[0] = PeerMessageTypeBatchFullChallenge
			_, err = parseNetworkMessage(TransportMessageVersion, retyped)
			require.Error(t, err)
		})
	}
}
