package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

const (
	snapshotsPrefixNodePledge = "NODESTATEPLEDGE"
	snapshotsPrefixNodeAccept = "NODESTATEACCEPT"
	snapshotsPrefixNodeDepart = "NODESTATEDEPART"
	snapshotsPrefixNodeRemove = "NODESTATEREMOVE"
)

func (s *BadgerStore) SnapshotsReadConsensusNodes() []common.Node {
	nodes := make([]common.Node, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	accepted := readNodesInState(txn, snapshotsPrefixNodeAccept)
	for _, n := range accepted {
		nodes = append(nodes, common.Node{Account: n, State: common.NodeStateAccepted})
	}
	pledging := readNodesInState(txn, snapshotsPrefixNodePledge)
	for _, n := range pledging {
		nodes = append(nodes, common.Node{Account: n, State: common.NodeStatePledging})
	}
	departing := readNodesInState(txn, snapshotsPrefixNodeDepart)
	for _, n := range departing {
		nodes = append(nodes, common.Node{Account: n, State: common.NodeStateDeparting})
	}
	return nodes
}

func readNodesInState(txn *badger.Txn, nodeState string) []common.Address {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(nodeState)
	nodes := make([]common.Address, 0)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		nodes = append(nodes, nodeAccountForState(it.Item().Key(), nodeState))
	}
	return nodes
}

func writeNodeAccept(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash, genesis bool) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodePledgeKey(publicSpend)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		if !genesis {
			return fmt.Errorf("node not pledging yet %s", publicSpend.String())
		}
	} else if err != nil {
		return err
	}

	key = nodeAcceptKey(publicSpend)
	return txn.Set(key, tx[:])
}

func writeNodePledge(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodeAcceptKey(publicSpend)
	_, err := txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already accepted %s", publicSpend.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	pledging := readNodesInState(txn, snapshotsPrefixNodePledge)
	if len(pledging) > 0 {
		node := pledging[0]
		return fmt.Errorf("node %s is pledging", node.PublicSpendKey.String())
	}

	departing := readNodesInState(txn, snapshotsPrefixNodeDepart)
	if len(departing) > 0 {
		node := departing[0]
		return fmt.Errorf("node %s is departing", node.PublicSpendKey.String())
	}

	key = nodePledgeKey(publicSpend)
	return txn.Set(key, tx[:])
}

func nodeAccountForState(key []byte, nodeState string) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(nodeState):])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func nodePledgeKey(publicSpend crypto.Key) []byte {
	return append([]byte(snapshotsPrefixNodePledge), publicSpend[:]...)
}

func nodeAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(snapshotsPrefixNodeAccept), publicSpend[:]...)
}

func nodeDepartKey(publicSpend crypto.Key) []byte {
	return append([]byte(snapshotsPrefixNodeDepart), publicSpend[:]...)
}
