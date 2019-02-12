package storage

import (
	"encoding/binary"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	graphPrefixGhost        = "GHOST"        // each output key should only be used once
	graphPrefixDeposit      = "DEPOSIT"      // unspent outputs, including first consumed transaction hash
	graphPrefixUTXO         = "UTXO"         // unspent outputs, including first consumed transaction hash
	graphPrefixTransaction  = "TRANSACTION"  // raw transaction, may not be finalized yet, if finalized with first finalized snapshot hash
	graphPrefixFinalization = "FINALIZATION" // transaction finalization hack
	graphPrefixUnique       = "UNIQUE"       // unique transaction in one node
	graphPrefixRound        = "ROUND"        // hash|node-if-cache {node:hash,number:734,references:{self-parent-round-hash,external-round-hash}}
	graphPrefixSnapshot     = "SNAPSHOT"     // {
	graphPrefixLink         = "LINK"         // self-external number
	graphPrefixNode         = "NODE"         // {head}
	graphPrefixTopology     = "TOPOLOGY"
)

func (s *BadgerStore) ReadSnapshotsForNodeRound(nodeId crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	snapshots := make([]*common.Snapshot, 0)

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	key := graphSnapshotKey(nodeId, round, crypto.Hash{})
	prefix := key[:len(key)-len(crypto.Hash{})]
	for it.Seek(key); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		var s common.Snapshot
		err = msgpack.Unmarshal(v, &s)
		if err != nil {
			return snapshots, err
		}
		snapshots = append(snapshots, &s)
	}

	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Timestamp < snapshots[j].Timestamp })
	return snapshots, nil
}

func (s *BadgerStore) WriteSnapshot(snap *common.SnapshotWithTopologicalOrder) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert only, remove in future
	if config.Debug {
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
		tx, err := readTransaction(txn, snap.Transaction.PayloadHash())
		if err != nil {
			return err
		}
		if tx == nil {
			panic("snapshot transaction not exist")
		}
		key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction.PayloadHash())
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	err := writeSnapshot(txn, snap)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func writeSnapshot(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder) error {
	err := finalizeTransaction(txn, &snap.Transaction.Transaction, snap.PayloadHash())
	if err != nil {
		return err
	}

	key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction.PayloadHash())
	val := common.MsgpackMarshalPanic(snap)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	key = graphUniqueKey(snap.NodeId, snap.Transaction.PayloadHash())
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	return writeTopology(txn, snap)
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

func graphSnapshotKey(nodeId crypto.Hash, round uint64, hash crypto.Hash) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, round)
	key := append([]byte(graphPrefixSnapshot), nodeId[:]...)
	key = append(key, buf...)
	return append(key, hash[:]...)
}
