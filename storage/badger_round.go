package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) SnapshotsReadNodesList() ([]crypto.Hash, error) {
	var nodes []crypto.Hash

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()

	filter := make(map[crypto.Hash]bool)
	for it.Seek([]byte(snapshotsPrefixNodeRound)); it.ValidForPrefix([]byte(snapshotsPrefixNodeRound)); it.Next() {
		var hash crypto.Hash
		key := it.Item().Key()
		id := key[len(snapshotsPrefixNodeRound) : len(snapshotsPrefixNodeRound)+len(hash)]
		copy(hash[:], id)
		if filter[hash] {
			continue
		}
		filter[hash] = true
		nodes = append(nodes, hash)
	}

	return nodes, nil
}

func (s *BadgerStore) SnapshotsReadRoundMeta(nodeIdWithNetwork crypto.Hash) ([2]uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readRoundMeta(txn, nodeIdWithNetwork)
}

func (s *BadgerStore) SnapshotsReadRoundLink(from, to crypto.Hash) (uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readRoundLink(txn, from, to)
}

func readRoundMeta(txn *badger.Txn, nodeIdWithNetwork crypto.Hash) ([2]uint64, error) {
	meta := [2]uint64{}
	key := nodeRoundMetaKey(nodeIdWithNetwork)
	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		return meta, nil
	}
	if err != nil {
		return meta, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return meta, err
	}
	number := binary.BigEndian.Uint64(ival[:8])
	start := binary.BigEndian.Uint64(ival[8:])
	meta[0], meta[1] = number, start
	return meta, nil
}

func writeRoundMeta(txn *badger.Txn, nodeIdWithNetwork crypto.Hash, number, start uint64) error {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, number)
	binary.BigEndian.PutUint64(buf[8:], start)
	key := nodeRoundMetaKey(nodeIdWithNetwork)
	return txn.Set(key, buf)
}

func readRoundLink(txn *badger.Txn, from, to crypto.Hash) (uint64, error) {
	key := nodeRoundLinkKey(from, to)
	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(ival), nil
}

func writeRoundLink(txn *badger.Txn, from, to crypto.Hash, link uint64) error {
	// TODO this old check is only an assert kind check, not needed at all
	old, err := readRoundLink(txn, from, to)
	if err != nil {
		return err
	}
	if old > link {
		return fmt.Errorf("invalid round link %d=>%d", old, link)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, link)
	key := nodeRoundLinkKey(from, to)
	return txn.Set([]byte(key), buf)
}

func nodeRoundMetaKey(nodeIdWithNetwork crypto.Hash) []byte {
	return append([]byte(snapshotsPrefixNodeRound), nodeIdWithNetwork[:]...)
}

func nodeRoundLinkKey(from, to crypto.Hash) []byte {
	link := crypto.NewHash(append(from[:], to[:]...))
	return append([]byte(snapshotsPrefixNodeLink), link[:]...)
}
