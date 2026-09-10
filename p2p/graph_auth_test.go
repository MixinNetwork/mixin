package p2p

import (
	"fmt"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestGraphMessageAdmission(t *testing.T) {
	sender := crypto.Blake3Hash([]byte("graph sender"))
	receiver := crypto.Blake3Hash([]byte("graph receiver"))
	relayer := crypto.Blake3Hash([]byte("graph relayer"))
	intermediate := crypto.Blake3Hash([]byte("intermediate graph relayer"))
	for _, route := range []string{"direct", "relayed", "nested relay"} {
		for _, accepted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/accepted=%t", route, accepted), func(t *testing.T) {
				handle := newP2PStubHandle(t)
				t.Cleanup(handle.cache.Close)
				handle.updateAccepted = accepted
				handle.consensusPeers = map[crypto.Hash]crypto.Key{sender: handle.key.Public()}
				me := NewPeer(handle, receiver, "test", false)
				neighbor := NewPeer(nil, sender, "test neighbor", true)
				me.relayers.Set(sender, neighbor)
				wire := buildGraphMessage(handle)
				peerID := sender
				if route != "direct" {
					wire = (&Peer{IdForNetwork: sender}).buildRelayMessage(receiver, wire)
					peerID = relayer
				}
				if route == "nested relay" {
					wire = (&Peer{IdForNetwork: intermediate}).buildRelayMessage(receiver, wire)
				}
				msg, err := parseNetworkMessage(TransportMessageVersion, wire)
				require.NoError(t, err)
				// Dropping one graph must not close the connection carrying it.
				require.NoError(t, me.handlePeerMessage(peerID, msg))
				require.Equal(t, sender, handle.updatePeerID)
				require.Equal(t, handle.graph, handle.updatePoints)
				if accepted {
					select {
					case graph := <-neighbor.syncRing:
						require.Equal(t, handle.graph, graph)
					default:
						t.Fatal("accepted graph did not reach peer sync")
					}
				} else {
					require.Empty(t, neighbor.syncRing, "rejected graph reached peer sync")
				}
			})
		}
	}
}

func TestGraphMessageNeighborSignature(t *testing.T) {
	sender := crypto.Blake3Hash([]byte("authenticated graph sender"))
	receiver := crypto.Blake3Hash([]byte("authenticated graph receiver"))
	relayer := crypto.Blake3Hash([]byte("forwarding graph relay"))
	attacker := p2pTestPrivateKey(93)
	for _, route := range []string{"direct", "relayed", "nested relay"} {
		for _, connection := range []string{"relayer", "consumer"} {
			for _, variant := range []string{"valid", "wrong signing key", "changed payload", "missing key", "wrong cached identity", "zero signature", "not permitted"} {
				t.Run(route+"/"+connection+"/"+variant, func(t *testing.T) {
					handle := newP2PStubHandle(t)
					t.Cleanup(handle.cache.Close)
					handle.consensusPeers = map[crypto.Hash]crypto.Key{}
					me := NewPeer(handle, receiver, "test", false)
					neighbor := NewPeer(nil, sender, "authenticated neighbor", connection == "relayer")
					neighbor.authentication = &AuthToken{PeerId: sender, PublicSpendKey: handle.key.Public()}
					if connection == "relayer" {
						me.relayers.Set(sender, neighbor)
					} else {
						me.consumers.Set(sender, neighbor)
					}
					wire := buildGraphMessage(handle)
					switch variant {
					case "wrong signing key":
						sig := attacker.Sign(crypto.Blake3Hash(wire[65:]))
						copy(wire[1:65], sig[:])
					case "changed payload":
						wire[len(wire)-1] ^= 1
					case "missing key":
						neighbor.authentication = nil
					case "wrong cached identity":
						neighbor.authentication.PeerId = relayer
					case "zero signature":
						clear(wire[1:65])
					case "not permitted":
						handle.updateAccepted = false
					}
					peerID := sender
					if route != "direct" {
						wire = (&Peer{IdForNetwork: sender}).buildRelayMessage(receiver, wire)
						peerID = relayer
					}
					if route == "nested relay" {
						wire = (&Peer{IdForNetwork: relayer}).buildRelayMessage(receiver, wire)
					}
					msg, err := parseNetworkMessage(TransportMessageVersion, wire)
					require.NoError(t, err)
					require.NoError(t, me.handlePeerMessage(peerID, msg))
					if variant == "valid" || variant == "not permitted" {
						require.Equal(t, sender, handle.updatePeerID)
					} else {
						require.Empty(t, handle.updatePoints, "unauthenticated graph reached the kernel")
					}
					if variant == "valid" {
						require.Len(t, neighbor.syncRing, 1)
					} else {
						require.Empty(t, neighbor.syncRing, "rejected graph reached peer sync")
					}
				})
			}
		}
	}
}
