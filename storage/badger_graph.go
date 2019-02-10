package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	graphPrefixGhost       = "GHOST"       // each output key should only be used once
	graphPrefixUTXO        = "UTXO"        // unspent outputs, including first consumed transaction hash
	graphPrefixTransaction = "TRANSACTION" // raw transaction, may not be finalized yet, if finalized with first finalized snapshot hash
	graphPrefixRound       = "ROUND"       // hash|node-if-cache {node:hash,number:734,references:{self-parent-round-hash,external-round-hash}}
	graphPrefixSnapshot    = "SNAPSHOT"    // {
	graphPrefixLink        = "LINK"        // self-external number
	graphPrefixNode        = "NODE"        // {head}
	graphPrefixTopology    = "TOPOLOGY"
)

func (s *BadgerStore) ReadTransaction(hash crypto.Hash) (*common.TransactionWithTopologicalOrder, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readTransaction(txn, hash)
}

func (s *BadgerStore) WriteTransaction(tx *common.TransactionWithTopologicalOrder) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	err := writeTransaction(txn, tx)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) ReadRound(hash crypto.Hash) (*common.Round, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readRound(txn, hash)
}

func (s *BadgerStore) StartNewRound(node crypto.Hash, number, start uint64, references [2]crypto.Hash) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	self, err := readRound(txn, node)
	if err != nil {
		return err
	}
	external, err := readRound(txn, references[1])
	if err != nil {
		return err
	}

	// FIXME assert only, remove in future
	if self == nil || self.Number != number-1 {
		panic("self final assert error")
	}
	if external == nil {
		panic("external final not exist")
	}
	old, err := readRound(txn, references[0])
	if err != nil {
		return err
	}
	if old != nil {
		panic("self final already exist")
	}
	link, err := readLink(txn, node, external.NodeId)
	if err != nil {
		return err
	}
	if link > external.Number {
		panic("external link backward")
	}
	// assert end

	err = writeLink(txn, node, external.NodeId, external.Number)
	if err != nil {
		return err
	}
	err = writeRound(txn, references[0], self)
	if err != nil {
		return err
	}
	err = writeRound(txn, node, &common.Round{
		NodeId:     node,
		Number:     number,
		Timestamp:  start,
		References: references,
	})
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (s *BadgerStore) WriteSnapshot(snap *common.Snapshot) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert only, remove in future
	cache, err := readRound(txn, snap.NodeId)
	if err != nil {
		return err
	}
	if cache == nil || snap.RoundNumber != cache.Number {
		panic("snapshot round number assert error")
	}
	if snap.References[0] != cache.References[0] || snap.References[1] != cache.References[1] {
		panic("snapshot references assert error")
	}
	// end assert

	tx, err := readTransaction(txn, snap.Transaction.PayloadHash())
	if err != nil {
		return err
	}
	if tx == nil {
		panic("snapshot transaction not exist")
	}
	if !tx.Snapshot.HasValue() {
		tx.Snapshot = snap.PayloadHash()
		err = writeTransaction(txn, tx)
		if err != nil {
			return err
		}
		err = writeTopology(txn, tx)
		if err != nil {
			return err
		}
	}

	key := graphSnapshotKey(snap.PayloadHash())
	val := common.MsgpackMarshalPanic(snap)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) ReadTransactionsSinceTopology(topologyOffset, count uint64) ([]*common.TransactionWithTopologicalOrder, error) {
	transactions := make([]*common.TransactionWithTopologicalOrder, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek(graphTopologyKey(topologyOffset))
	for ; it.ValidForPrefix([]byte(graphPrefixTopology)) && uint64(len(transactions)) < count; it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return transactions, err
		}
		var hash crypto.Hash
		copy(hash[:], v)
		tx, err := readTransaction(txn, hash)
		if err != nil {
			return transactions, err
		}
		tx.TopologicalOrder = graphTopologyOrder(item.Key())
		tx.Hash = tx.PayloadHash()
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func (s *BadgerStore) TopologySequence() uint64 {
	var sequence uint64

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(topologyKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixTopology)) {
		item := it.Item()
		sequence = topologyOrder(item.Key()) + 1
	}
	return sequence
}

func writeTopology(txn *badger.Txn, tx *common.TransactionWithTopologicalOrder) error {
	key := graphTopologyKey(tx.TopologicalOrder)
	val := tx.Snapshot[:]
	return txn.Set(key, val)
}

func readTransaction(txn *badger.Txn, hash crypto.Hash) (*common.TransactionWithTopologicalOrder, error) {
	var out common.TransactionWithTopologicalOrder
	key := graphTransactionKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeTransaction(txn *badger.Txn, tx *common.TransactionWithTopologicalOrder) error {
	key := graphTransactionKey(tx.PayloadHash())
	val := common.MsgpackMarshalPanic(tx)
	return txn.Set(key, val)
}

func readLink(txn *badger.Txn, from, to crypto.Hash) (uint64, error) {
	key := graphLinkKey(from, to)
	item, err := txn.Get(key)
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

func writeLink(txn *badger.Txn, from, to crypto.Hash, link uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, link)
	key := graphLinkKey(from, to)
	return txn.Set(key, buf)
}

func readRound(txn *badger.Txn, hash crypto.Hash) (*common.Round, error) {
	var out common.Round
	key := graphRoundKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeRound(txn *badger.Txn, hash crypto.Hash, round *common.Round) error {
	key := graphRoundKey(hash)
	val := common.MsgpackMarshalPanic(round)
	return txn.Set(key, val)
}

func graphReadValue(txn *badger.Txn, key []byte, val interface{}) error {
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return err
	}
	return msgpack.Unmarshal(ival, &val)
}

func graphRoundKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixRound), hash[:]...)
}

func graphTransactionKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixTransaction), hash[:]...)
}

func graphSnapshotKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixSnapshot), hash[:]...)
}

func graphLinkKey(from, to crypto.Hash) []byte {
	link := crypto.NewHash(append(from[:], to[:]...))
	return append([]byte(graphPrefixLink), link[:]...)
}

func graphTopologyKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte(graphPrefixTopology), buf...)
}

func graphTopologyOrder(key []byte) uint64 {
	order := key[len(graphPrefixTopology):]
	return binary.BigEndian.Uint64(order)
}
