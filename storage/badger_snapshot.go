package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	snapshotsPrefixSnapshot = "SNAPSHOT" // transaction hash to snapshot meta, mainly node and consensus timestamp
	snapshotsPrefixGraph    = "GRAPH"    // consensus directed asyclic graph data store
	snapshotsPrefixUTXO     = "UTXO"     // unspent outputs, will be deleted once consumed
	snapshotsPrefixNode     = "NODE"     // node specific info, e.g. round number, round hash
	snapshotsPrefixGhost    = "GHOST"    // each output key should only be used once
)

func (s *BadgerStore) SnapshotsNodeList() ([]crypto.Hash, error) {
	var nodes []crypto.Hash
	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		filter := make(map[string]bool)
		for it.Seek([]byte(snapshotsPrefixNode)); it.ValidForPrefix([]byte(snapshotsPrefixNode)); it.Next() {
			var hash crypto.Hash
			key := it.Item().Key()
			id := key[len(snapshotsPrefixNode) : len(snapshotsPrefixNode)+len(hash)]
			copy(hash[:], id)
			if filter[hash.String()] {
				continue
			}
			filter[hash.String()] = true
			nodes = append(nodes, hash)
		}

		return nil
	})
	return nodes, err
}

func (s *BadgerStore) SnapshotsRoundMetaForNode(nodeIdWithNetwork crypto.Hash) (uint64, uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readNodeRoundMeta(txn, nodeIdWithNetwork)
}

func readNodeRoundMeta(txn *badger.Txn, nodeIdWithNetwork crypto.Hash) (uint64, uint64, error) {
	key := nodeRoundMetaKey(nodeIdWithNetwork)
	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	ival, err := item.Value()
	if err != nil {
		return 0, 0, err
	}
	number := binary.BigEndian.Uint64(ival[:8])
	start := binary.BigEndian.Uint64(ival[8:])
	return number, start, nil
}

func (s *BadgerStore) SnapshotsListForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	snapshots := make([]*common.Snapshot, 0)
	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		key := graphKey(nodeIdWithNetwork, round, 0)
		prefix := key[:len(key)-8]
		for it.Seek(key); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			v, err := item.Value()
			if err != nil {
				return err
			}
			var s common.Snapshot
			err = msgpack.Unmarshal(v, &s)
			if err != nil {
				return err
			}
			snapshots = append(snapshots, &s)
		}
		return nil
	})
	return snapshots, err
}

func (s *BadgerStore) SnapshotsGetUTXO(hash crypto.Hash, index int) (*common.UTXO, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := utxoKey(hash, index)
	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ival, err := item.Value()
	if err != nil {
		return nil, err
	}
	var utxo common.UTXO
	err = msgpack.Unmarshal(ival, &utxo)
	return &utxo, err
}

func (s *BadgerStore) SnapshotsCheckGhost(key crypto.Key) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	_, err := txn.Get([]byte(ghostKey(key)))
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) SnapshotsLoadGenesis(snapshots []*common.Snapshot) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		if checkGenesisLoad(txn) {
			return nil
		}

		for _, snap := range snapshots {
			err := saveSnapshot(s.seqSource, txn, snap, true)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func checkGenesisLoad(txn *badger.Txn) bool {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Rewind()
	return it.Valid()
}

func readSnapshotByTransactionHash(txn *badger.Txn, hash crypto.Hash) (*common.Snapshot, error) {
	item, err := txn.Get(snapshotKey(hash))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	meta, err := item.Value()
	if err != nil {
		return nil, err
	}

	key := meta[:len(graphKey(crypto.Hash{}, 0, 0))]
	item, err = txn.Get(key)
	if err == badger.ErrKeyNotFound {
		panic(hash.String())
	} else if err != nil {
		return nil, err
	}
	val, err := item.Value()
	if err != nil {
		return nil, err
	}
	var s common.Snapshot
	err = msgpack.Unmarshal(val, &s)
	return &s, err
}

func saveSnapshot(topoCounter *TopologicalSequence, txn *badger.Txn, snapshot *common.Snapshot, genesis bool) error {
	_, err := txn.Get(snapshotKey(snapshot.Transaction.Hash()))
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	roundNumber, roundStart, err := readNodeRoundMeta(txn, snapshot.NodeId)
	if err != nil {
		return err
	}
	if snapshot.RoundNumber < roundNumber {
		return ErrorValidateFailed
	}
	if snapshot.RoundNumber == roundNumber && (snapshot.Timestamp-roundStart) >= 3*1e9 {
		return ErrorValidateFailed
	}

	for _, in := range snapshot.Transaction.Inputs {
		if genesis && in.Hash.String() == (crypto.Hash{}).String() {
			continue
		}
		key := utxoKey(in.Hash, in.Index)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return ErrorValidateFailed
		} else if err != nil {
			return err
		}
		err = txn.Delete(key)
		if err != nil {
			return err
		}
	}

	for _, utxo := range snapshot.UnspentOutputs() {
		for _, k := range utxo.Keys {
			key := ghostKey(k)
			_, err := txn.Get(key)
			if err == nil {
				return ErrorValidateFailed
			} else if err != badger.ErrKeyNotFound {
				return err
			}
			err = txn.Set(key, []byte{0})
			if err != nil {
				return err
			}
		}
		key := utxoKey(utxo.Hash, utxo.Index)
		val := common.MsgpackMarshalPanic(utxo)
		err = txn.Set(key, val)
		if err != nil {
			return err
		}
	}

	key := graphKey(snapshot.NodeId, snapshot.RoundNumber, snapshot.Timestamp)
	_, err = txn.Get(key)
	if err == nil {
		return ErrorValidateFailed
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	val := common.MsgpackMarshalPanic(snapshot)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	meta := append(key, byte(len(snapshot.References)))
	for _, ref := range snapshot.References {
		meta = append(meta, ref[:]...)
	}
	err = txn.Set(snapshotKey(snapshot.Transaction.Hash()), meta)
	if err != nil {
		return err
	}

	// not related to consensus
	seq := topoCounter.Next()
	return saveSnapshotTopology(txn, snapshot, seq)
}

func snapshotKey(transactionHash crypto.Hash) []byte {
	return append([]byte(snapshotsPrefixSnapshot), transactionHash[:]...)
}

func graphKey(nodeIdWithNetwork crypto.Hash, round, ts uint64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, round)
	binary.BigEndian.PutUint64(buf, ts)
	key := append([]byte(snapshotsPrefixGraph), nodeIdWithNetwork[:]...)
	return append(key, buf...)
}

func utxoKey(hash crypto.Hash, index int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte(snapshotsPrefixUTXO), hash[:]...)
	return append(key, buf[:size]...)
}

func nodeRoundMetaKey(nodeIdWithNetwork crypto.Hash) []byte {
	key := append([]byte(snapshotsPrefixNode), nodeIdWithNetwork[:]...)
	return append(key, []byte("ROUND")...)
}

func ghostKey(k crypto.Key) []byte {
	return append([]byte(snapshotsPrefixGhost), k[:]...)
}
