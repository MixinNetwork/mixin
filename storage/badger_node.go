package storage

import (
	"encoding/binary"
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

func (s *BadgerStore) SnapshotsReadAcceptedNodes() ([]common.Address, error) {
	nodes := make([]common.Address, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(snapshotsPrefixNodeAccept)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		nodes = append(nodes, nodeAcceptAccount(item.Key()))
	}

	return nodes, nil
}

func writeNodePledge(txn *badger.Txn, publicSpend crypto.Key, snapshotTimestamp uint64) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodePledgeKey(publicSpend)
	_, err := txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already pledged %s", publicSpend.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	key = nodeAcceptKey(publicSpend)
	_, err = txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already accepted %s", publicSpend.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	key = nodeDepartKey(publicSpend)
	_, err = txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already departed%s", publicSpend.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	key = nodePledgeKey(publicSpend)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, snapshotTimestamp)
	return txn.Set(key, buf)
}

func nodeAcceptAccount(key []byte) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(snapshotsPrefixNodeAccept):])
	seed := crypto.NewHash(publicSpend[:])
	privateView := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
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
