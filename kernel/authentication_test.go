package kernel

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestAuthenticateAsReturnsVerifiedPublicKey(t *testing.T) {
	consumer := newAuthenticationTestNode(91, false)
	relayer := newAuthenticationTestNode(92, true)
	for _, pair := range [][2]*Node{{consumer, relayer}, {relayer, consumer}} {
		sender, recipient := pair[0], pair[1]
		message := sender.BuildAuthenticationMessage(recipient.IdForNetwork)
		token, err := recipient.AuthenticateAs(recipient.IdForNetwork, message, 10)
		require.NoError(t, err)
		require.Equal(t, sender.IdForNetwork, token.PeerId)
		require.Equal(t, sender.Signer.PublicSpendKey, token.PublicSpendKey)
		require.Equal(t, sender.isRelayer, token.IsRelayer)
		require.Equal(t, message, token.Data)
		payload := []byte("graph signed after authentication")
		sig := sender.SignData(payload)
		require.True(t, token.PublicSpendKey.Verify(crypto.Blake3Hash(payload), sig))
		message[40] ^= 1
		require.NotEqual(t, message, token.Data, "authentication must retain its own copy")
	}
}

func TestAuthenticateAsRejectsInvalidKeyExchange(t *testing.T) {
	sender := newAuthenticationTestNode(93, true)
	recipient := newAuthenticationTestNode(94, false)
	other := newAuthenticationTestNode(95, true)
	for _, variant := range []string{"wrong recipient", "changed key", "wrong signature", "stale", "truncated", "self"} {
		t.Run(variant, func(t *testing.T) {
			message := sender.BuildAuthenticationMessage(recipient.IdForNetwork)
			switch variant {
			case "wrong recipient":
				message = sender.BuildAuthenticationMessage(other.IdForNetwork)
			case "changed key":
				copy(message[40:72], other.Signer.PublicSpendKey[:])
			case "wrong signature":
				sig := other.SignData(message[:73])
				copy(message[73:], sig[:])
			case "stale":
				binary.BigEndian.PutUint64(message[:8], uint64(clock.Now().Add(-time.Minute).Unix()))
				sig := sender.SignData(message[:73])
				copy(message[73:], sig[:])
			case "truncated":
				message = message[:len(message)-1]
			case "self":
				message = recipient.BuildAuthenticationMessage(recipient.IdForNetwork)
			}
			token, err := recipient.AuthenticateAs(recipient.IdForNetwork, message, 10)
			require.Error(t, err)
			require.Nil(t, token, "invalid authentication must not expose a verified key")
		})
	}
}

func newAuthenticationTestNode(seed byte, relayer bool) *Node {
	node := &Node{custom: &config.Custom{}, networkId: crypto.Blake3Hash([]byte("authentication test network"))}
	node.custom.Node.Signer = crypto.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 64))
	node.custom.P2P.Relayer = relayer
	node.loadNodeConfig()
	node.IdForNetwork = node.Signer.Hash().ForNetwork(node.networkId)
	return node
}
