package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

const (
	snapshotsPrefixNodePledge = "NODEPLEDGE"
	snapshotsPrefixNodeAccept = "NODEACCEPT"
	snapshotsPrefixNodeDepart = "NODEDEPART"
	snapshotsPrefixNodeRemove = "NODEREMOVE"
)

func (s *BadgerStore) SnapshotsCheckPendingNodes() bool {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	pit := txn.NewIterator(badger.DefaultIteratorOptions)
	defer pit.Close()
	prefix := []byte(snapshotsPrefixNodePledge)
	pit.Seek(prefix)
	if pit.ValidForPrefix(prefix) {
		return true
	}
	pit.Close()

	dit := txn.NewIterator(badger.DefaultIteratorOptions)
	defer dit.Close()
	prefix = []byte(snapshotsPrefixNodeDepart)
	dit.Seek(prefix)
	return dit.ValidForPrefix(prefix)
}

func (s *BadgerStore) SnapshotsReadAcceptedNodes() ([]common.Address, error) {
	nodes := make([]common.Address, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(snapshotsPrefixNodeAccept)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		nodes = append(nodes, nodeAcceptAccount(it.Item().Key()))
	}

	return nodes, nil
}

func writeNodePledge(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash, genesis bool) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodeAcceptKey(publicSpend)
	_, err := txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already accepted %s", publicSpend.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	pit := txn.NewIterator(badger.DefaultIteratorOptions)
	defer pit.Close()
	prefix := []byte(snapshotsPrefixNodePledge)
	pit.Seek(prefix)
	if pit.ValidForPrefix(prefix) && !genesis {
		node := nodePledgeAccount(pit.Item().Key())
		return fmt.Errorf("node %s is pledging", node.PublicSpendKey.String())
	}
	pit.Close()

	dit := txn.NewIterator(badger.DefaultIteratorOptions)
	defer dit.Close()
	prefix = []byte(snapshotsPrefixNodeDepart)
	dit.Seek(prefix)
	if dit.ValidForPrefix(prefix) {
		node := nodeDepartAccount(dit.Item().Key())
		return fmt.Errorf("node %s is departing", node.PublicSpendKey.String())
	}

	key = nodePledgeKey(publicSpend)
	return txn.Set(key, tx[:])
}

func nodePledgeAccount(key []byte) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(snapshotsPrefixNodePledge):])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func nodeAcceptAccount(key []byte) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(snapshotsPrefixNodeAccept):])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func nodeDepartAccount(key []byte) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(snapshotsPrefixNodeDepart):])
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
