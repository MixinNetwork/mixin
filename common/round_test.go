package common

import (
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestRound(t *testing.T) {
	require := require.New(t)

	round := &Round{
		Hash:      crypto.Blake3Hash([]byte("hello-round-hash")),
		NodeId:    crypto.Blake3Hash([]byte("hello-round-node")),
		Number:    123,
		Timestamp: 456,
		References: &RoundLink{
			Self:     crypto.Blake3Hash([]byte("self-link")),
			External: crypto.Blake3Hash([]byte("external-link")),
		},
	}

	rb := round.CompressMarshal()
	require.Equal("0000000028b52ffd0300c118533cb1040077770001cab134dd2dc41a86cdd207d53bef35e5d5b04d6f20af5b86b3acd376fea66f9c776b9418b60941e9eb4111e6941af381657f17ffeeecca505cd94bdf1d7c7f83000000000000007b00000000000001c800022d255d71898c1d7975c0fe3e18eb67af3fa28976b1ca26bc87235c8b565716ea8436a1dc1b70b6f3c6a65dac7920e886417d21b52439970535df2e7d25570063", hex.EncodeToString(rb))

	un, err := DecompressUnmarshalRound(rb)
	require.Nil(err)
	require.Equal("cab134dd2dc41a86cdd207d53bef35e5d5b04d6f20af5b86b3acd376fea66f9c", un.Hash.String())
}
