package kernel

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/stretchr/testify/require"
)

type custodianApprovalStore struct {
	storage.Store
	current *common.CustodianUpdateRequest
	nodes   []*common.Node
	readAt  uint64
}

func (s *custodianApprovalStore) ReadCustodian(ts uint64) (*common.CustodianUpdateRequest, error) {
	s.readAt = ts
	return s.current, nil
}

func (s *custodianApprovalStore) ReadAllNodes(uint64, bool) []*common.Node {
	return s.nodes
}

func TestCustodianUpdateApprovalUsesStoredPredecessor(t *testing.T) {
	address := func(seed byte) common.Address {
		a := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
		a.PrivateViewKey = a.PublicSpendKey.DeterministicHashDerive()
		a.PublicViewKey = a.PrivateViewKey.Public()
		return a
	}
	a, b := address(1), address(2)
	network := crypto.Blake3Hash([]byte("kernel custodian approval test"))
	store := &custodianApprovalStore{}
	var entries [][]byte
	var members []*CNode
	for i := range config.KernelMinimumNodesCount {
		signer, payee, custodian := address(byte(10+i*3)), address(byte(11+i*3)), address(byte(12+i*3))
		n := &common.Node{Signer: signer, Payee: payee, State: common.NodeStateAccepted}
		store.nodes = append(store.nodes, n)
		members = append(members, &CNode{IdForNetwork: n.IdForNetwork(network), Signer: signer, Payee: payee, State: n.State})
		entries = append(entries, common.EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, network))
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i][1:33], entries[j][1:33]) < 0
	})
	unsigned := append([]byte{}, b.PublicSpendKey[:]...)
	unsigned = append(unsigned, b.PublicViewKey[:]...)
	for _, entry := range entries {
		unsigned = append(unsigned, entry...)
	}
	node := &Node{
		Epoch: 1, networkId: network, persistStore: store,
		acceptedNodeStateSequences: []*NodeStateSequence{{NodesWithoutState: members}},
	}
	ts := node.Epoch + uint64(17*time.Hour)
	snapshot := &common.Snapshot{Timestamp: ts, NodeId: node.electSnapshotNode(common.TransactionTypeCustodianUpdateNodes, ts)}
	previous := crypto.Blake3Hash([]byte("current A update"))
	old := crypto.Blake3Hash([]byte("historical A update"))
	for _, tc := range []struct {
		name   string
		stored crypto.Hash
		signed crypto.Hash
		valid  bool
	}{
		{"no previous update", crypto.Hash{}, crypto.Hash{}, true},
		{"current predecessor", previous, previous, true},
		{"missing predecessor binding", previous, crypto.Hash{}, false},
		{"historical approval with same custodian key", previous, old, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store.current = &common.CustodianUpdateRequest{Custodian: &a, Transaction: tc.stored}
			sig := a.PrivateSpendKey.Sign(common.CustodianUpdateNodesApprovalHash(unsigned, tc.signed))
			tx := common.NewTransactionV5(common.XINAssetId)
			tx.Extra = append(bytes.Clone(unsigned), sig[:]...)
			original := bytes.Clone(tx.Extra)
			for _, finalized := range []bool{false, true} {
				err := node.validateCustodianUpdateNodes(snapshot, tx.AsVersioned(), finalized)
				if tc.valid {
					require.NoError(t, err)
				} else {
					require.ErrorContains(t, err, "approval signature")
				}
				require.Equal(t, ts, store.readAt)
				require.Equal(t, original, tx.Extra)
			}
		})
	}
}
