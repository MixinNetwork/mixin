package p2p

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestParseNetworkMessageCommitmentBatchBoundaries(t *testing.T) {
	handle := newP2PStubHandle(t)
	commitments := make([]*crypto.CosiCommitment, 1024)
	for i := range commitments {
		var seed [64]byte
		binary.BigEndian.PutUint64(seed[:], uint64(2*i+1))
		r1 := crypto.NewKeyFromSeed(seed[:]).Public()
		binary.BigEndian.PutUint64(seed[:], uint64(2*i+2))
		r2 := crypto.NewKeyFromSeed(seed[:]).Public()
		commitment := crypto.NewCosiCommitment(r1, r2)
		commitments[i] = &commitment
	}

	// The final pair in a maximum-size batch starts beyond the uint16 range.
	for _, count := range []int{1, 512, 1023, 1024} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			wire := buildCommitmentsMessage(handle, commitments[:count])
			parsed, err := parseNetworkMessage(TransportMessageVersion, wire)
			require.NoError(t, err)
			require.EqualValues(t, PeerMessageTypePreCommitments, parsed.Type)
			require.Equal(t, commitments[:count], parsed.Commitments)
		})
	}
}

func TestParseNetworkMessageRejectsFullChallengeWithoutCosiSignature(t *testing.T) {
	require := require.New(t)
	handle := newP2PStubHandle(t)

	s := &common.Snapshot{
		Version:     common.SnapshotVersionCommonEncoding,
		NodeId:      crypto.Blake3Hash([]byte("node")),
		RoundNumber: 1,
		References: &common.RoundLink{
			Self:     crypto.Blake3Hash([]byte("self")),
			External: crypto.Blake3Hash([]byte("external")),
		},
		Timestamp: 2,
	}
	s.AddTransaction(crypto.Blake3Hash([]byte("tx")))

	seed := make([]byte, 64)
	seed[0] = 1
	commitment := crypto.NewCosiCommitment(crypto.NewKeyFromSeed(seed).Public(), crypto.NewKeyFromSeed(seed).Public())
	seed[0] = 2
	challenge := crypto.NewCosiCommitment(crypto.NewKeyFromSeed(seed).Public(), crypto.NewKeyFromSeed(seed).Public())
	seed[0] = 3
	randoms := crypto.NewCosiCommitment(crypto.NewKeyFromSeed(seed).Public(), crypto.NewKeyFromSeed(seed).Public())

	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	msg := buildBatchFullChallengeMessage(handle, s, &commitment, &challenge, &randoms, []*common.VersionedTransaction{tx})

	parsed, err := parseNetworkMessage(0, msg)
	require.Nil(parsed)
	require.ErrorContains(err, "invalid full challenge snapshot signature")
}
