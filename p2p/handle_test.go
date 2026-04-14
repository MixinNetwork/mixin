package p2p

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestParseNetworkMessageRejectsUnsignedFullChallenge(t *testing.T) {
	require := require.New(t)

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
	s.AddSoleTransaction(crypto.Blake3Hash([]byte("tx")))

	seed := make([]byte, 64)
	seed[0] = 1
	commitment := crypto.NewKeyFromSeed(seed).Public()
	seed[0] = 2
	challenge := crypto.NewKeyFromSeed(seed).Public()

	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	msg := buildFullChallengeMessage(s, &commitment, &challenge, tx)

	parsed, err := parseNetworkMessage(0, msg)
	require.Nil(parsed)
	require.ErrorContains(err, "invalid full challenge snapshot signature")
}
