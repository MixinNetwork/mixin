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
	require.Equal("0000000028b52ffd0300c118533cb104007777000141fcf8ecefae9071c8e9c779bdad09bd2f27a1edccd4df012e929a71a7cef15e3b4918d62413a03a21a89b486a41017f6b2776a9a72ba2c34605011e110bda1d000000000000007b00000000000001c8000200b75137647312fd194412a1df91fce7c9005743953a373613b08bd8e2827dcc8585f24ace862a82a0f2feb2cab0f893f5f267969ed0d71b4ecb7cb2387e016a", hex.EncodeToString(rb))

	un, err := DecompressUnmarshalRound(rb)
	require.Nil(err)
	require.Equal("41fcf8ecefae9071c8e9c779bdad09bd2f27a1edccd4df012e929a71a7cef15e", un.Hash.String())
}
