package kernel

import (
	"bytes"
	"encoding/binary"
	"testing"

	"filippo.io/edwards25519"
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestVerifyConsensusPeerSignatureRejectsForgedLeaderResponse(t *testing.T) {
	leaderPrivate := crypto.NewKeyFromSeed(bytes.Repeat([]byte{41}, 64))
	leaderPublic := leaderPrivate.Public()
	leaderID := crypto.Blake3Hash(leaderPublic[:])
	peer := &CNode{
		IdForNetwork: leaderID,
		Signer:       common.Address{PublicSpendKey: leaderPublic},
		State:        common.NodeStateAccepted,
	}
	node := &Node{
		IdForNetwork:       crypto.Blake3Hash([]byte("full challenge recipient")),
		nodeStateSequences: []*NodeStateSequence{{NodesWithoutState: []*CNode{peer}}},
	}

	snapshot := &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       leaderID,
		RoundNumber:  1,
		Timestamp:    1,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("full challenge transaction"))},
	}
	public := crypto.NewKeyFromSeed(bytes.Repeat([]byte{42}, 64)).Public()
	commitment := crypto.NewKeyFromSeed(bytes.Repeat([]byte{43}, 64)).Public()
	cosi, err := crypto.CosiAggregateCommitment(map[int]*crypto.Key{0: &commitment})
	require.NoError(t, err)
	challenge, err := cosi.Challenge([]*crypto.Key{&public}, snapshot.PayloadHash())
	require.NoError(t, err)

	// A full challenge supplies the leader's commitment along with its response.
	// Given c, choose s = 1 and R = sB - cA. The partial response verifies even
	// though the relay has never used the leader's private key.
	scalarBytes := [32]byte{1}
	response, err := edwards25519.NewScalar().SetCanonicalBytes(scalarBytes[:])
	require.NoError(t, err)
	leaderPoint, err := edwards25519.NewIdentityPoint().SetBytes(leaderPublic[:])
	require.NoError(t, err)
	forgedPoint := edwards25519.NewIdentityPoint().VarTimeDoubleScalarBaseMult(
		challenge, edwards25519.NewIdentityPoint().Negate(leaderPoint), response,
	)
	var forgedCommitment crypto.Key
	copy(forgedCommitment[:], forgedPoint.Bytes())
	copy(cosi.Signature[32:], response.Bytes())
	var partial crypto.Signature
	copy(partial[:32], forgedCommitment[:])
	copy(partial[32:], response.Bytes())
	require.True(t, forgedCommitment.CheckKey())
	require.True(t, leaderPublic.VerifyWithChallenge(partial, challenge))
	require.False(t, leaderPublic.Verify(snapshot.PayloadHash(), partial))

	signedSnapshot := *snapshot
	signedSnapshot.Signature = cosi
	payload := signedSnapshot.VersionedMarshal()
	data := binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	data = append(data, payload...)
	data = append(data, forgedCommitment[:]...)
	data = append(data, commitment[:]...)
	data = append(data, 0) // no transaction bodies
	attacker := crypto.NewKeyFromSeed(bytes.Repeat([]byte{44}, 64))
	attackerSignature := attacker.Sign(crypto.Blake3Hash(data))
	for _, signature := range []*crypto.Signature{nil, &partial, &attackerSignature} {
		require.False(t, node.VerifyConsensusPeerSignature(leaderID, data, signature),
			"a forged leader response must not authorize a full challenge")
		require.Equal(t, crypto.Hash{}, snapshot.Hash, "authentication must precede snapshot processing")
	}

	// The claimed leader must explicitly authorize the complete challenge.
	signature := leaderPrivate.Sign(crypto.Blake3Hash(data))
	require.True(t, node.VerifyConsensusPeerSignature(leaderID, data, &signature))
}

func TestVerifyConsensusPeerSignature(t *testing.T) {
	accepted := crypto.NewKeyFromSeed(bytes.Repeat([]byte{51}, 64))
	pledging := crypto.NewKeyFromSeed(bytes.Repeat([]byte{52}, 64))
	removed := crypto.NewKeyFromSeed(bytes.Repeat([]byte{53}, 64))
	acceptedID := crypto.Blake3Hash([]byte("accepted signature peer"))
	pledgingID := crypto.Blake3Hash([]byte("pledging signature peer"))
	removedID := crypto.Blake3Hash([]byte("removed signature peer"))
	unknownID := crypto.Blake3Hash([]byte("unknown signature peer"))
	node := &Node{nodeStateSequences: []*NodeStateSequence{{NodesWithoutState: []*CNode{
		{IdForNetwork: acceptedID, State: common.NodeStateAccepted, Signer: common.Address{PublicSpendKey: accepted.Public()}},
		{IdForNetwork: pledgingID, State: common.NodeStatePledging, Signer: common.Address{PublicSpendKey: pledging.Public()}},
		{IdForNetwork: removedID, State: common.NodeStateRemoved, Signer: common.Address{PublicSpendKey: removed.Public()}},
	}}}}
	data := []byte("signed consensus message")
	acceptedSig := accepted.Sign(crypto.Blake3Hash(data))
	pledgingSig := pledging.Sign(crypto.Blake3Hash(data))
	removedSig := removed.Sign(crypto.Blake3Hash(data))
	for _, test := range []struct {
		name string
		peer crypto.Hash
		data []byte
		sig  *crypto.Signature
		want bool
	}{
		{name: "accepted", peer: acceptedID, data: data, sig: &acceptedSig, want: true},
		{name: "pledging", peer: pledgingID, data: data, sig: &pledgingSig, want: true},
		{name: "removed", peer: removedID, data: data, sig: &removedSig},
		{name: "unknown", peer: unknownID, data: data, sig: &acceptedSig},
		{name: "wrong peer", peer: pledgingID, data: data, sig: &acceptedSig},
		{name: "changed payload", peer: acceptedID, data: []byte("changed consensus message"), sig: &acceptedSig},
		{name: "missing signature", peer: acceptedID, data: data},
		{name: "zero signature", peer: acceptedID, data: data, sig: &crypto.Signature{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, node.VerifyConsensusPeerSignature(test.peer, test.data, test.sig))
		})
	}
}
