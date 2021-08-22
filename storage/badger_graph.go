package storage

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

const (
	graphPrefixGhost        = "GHOST" // each output key should only be used once
	graphPrefixUTXO         = "UTXO"  // unspent outputs, including first consumed transaction hash
	graphPrefixDeposit      = "DEPOSIT"
	graphPrefixMint         = "MINT"
	graphPrefixTransaction  = "TRANSACTION"  // raw transaction, may not be finalized yet, if finalized with first finalized snapshot hash
	graphPrefixFinalization = "FINALIZATION" // transaction finalization hack
	graphPrefixUnique       = "UNIQUE"       // unique transaction in one node
	graphPrefixRound        = "ROUND"        // hash|node-if-cache {node:hash,number:734,references:{self-parent-round-hash,external-round-hash}}
	graphPrefixSnapshot     = "SNAPSHOT"     //
	graphPrefixLink         = "LINK"         // self-external number
	graphPrefixTopology     = "TOPOLOGY"
	graphPrefixSnapTopology = "SNAPTOPO"
	graphPrefixWorkLead     = "WORKPROPOSE"
	graphPrefixWorkSign     = "WORKVOTE"
	graphPrefixWorkOffset   = "WORKCHECKPOINT"
	graphPrefixWorkSnapshot = "WORKSNAPSHOT"
)

func (s *BadgerStore) RemoveGraphEntries(prefix string) (int, error) {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = []byte(prefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	var removed int
	it.Seek([]byte(prefix))
	for ; it.Valid(); it.Next() {
		key := it.Item().KeyCopy(nil)
		err := txn.Delete(key)
		if err != nil {
			return 0, err
		}
		removed += 1
	}
	it.Close()

	return removed, txn.Commit()
}

func (s *BadgerStore) ReadSnapshotsForNodeRound(nodeId crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readSnapshotsForNodeRound(txn, nodeId, round)
}

func readSnapshotsForNodeRound(txn *badger.Txn, nodeId crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 0)

	key := graphSnapshotKey(nodeId, round, crypto.Hash{})
	prefix := key[:len(key)-len(crypto.Hash{})]
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Seek(key); it.Valid(); it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		var s common.SnapshotWithTopologicalOrder
		err = common.DecompressMsgpackUnmarshal(v, &s)
		if err != nil {
			return snapshots, err
		}
		s.Hash = s.PayloadHash()
		snapshots = append(snapshots, &s)
	}

	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Timestamp < snapshots[j].Timestamp })
	return snapshots, nil
}

func (s *BadgerStore) WriteSnapshot(snap *common.SnapshotWithTopologicalOrder, signers []crypto.Hash) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert only, remove in future
	if config.Debug {
		cache, err := readRound(txn, snap.NodeId)
		if err != nil {
			return err
		}
		if cache == nil || snap.RoundNumber != cache.Number {
			panic(fmt.Errorf("snapshot round number assert error %d %d", cache.Number, snap.RoundNumber))
		}
		if snap.RoundNumber > 0 && !snap.References.Equal(cache.References) {
			panic("snapshot references assert error")
		}
		ver, err := readTransaction(txn, snap.Transaction)
		if err != nil {
			return err
		}
		if ver == nil {
			panic("snapshot transaction not exist")
		}
		key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction)
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
		key = graphUniqueKey(snap.NodeId, snap.Transaction)
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	ver, err := readTransaction(txn, snap.Transaction)
	if err != nil {
		return err
	}
	err = writeSnapshot(txn, snap, ver)
	if err != nil {
		return err
	}
	err = writeSnapshotWork(txn, snap, signers)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func writeSnapshot(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder, ver *common.VersionedTransaction) error {
	err := finalizeTransaction(txn, ver, snap)
	if err != nil {
		return err
	}

	key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction)
	val := common.CompressMsgpackMarshalPanic(snap)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	key = graphUniqueKey(snap.NodeId, snap.Transaction)
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	return writeTopology(txn, snap)
}

func graphSnapshotKey(nodeId crypto.Hash, round uint64, hash crypto.Hash) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, round)
	key := append([]byte(graphPrefixSnapshot), nodeId[:]...)
	key = append(key, buf...)
	return append(key, hash[:]...)
}
