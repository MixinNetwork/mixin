package storage

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v4"
)

const (
	graphPrefixGhost             = "GHOST" // each output key should only be used once
	graphPrefixUTXO              = "UTXO"  // unspent outputs, including first consumed transaction hash
	graphPrefixDeposit           = "DEPOSIT"
	graphPrefixWithdrawal        = "WITHDRAWAL"
	graphPrefixMint              = "MINTUNIVERSAL"
	graphPrefixTransaction       = "TRANSACTION"  // raw transaction, may not be finalized yet, if finalized with first finalized snapshot hash
	graphPrefixFinalization      = "FINALIZATION" // transaction finalization hack
	graphPrefixUnique            = "UNIQUE"       // unique transaction in one node
	graphPrefixRound             = "ROUND"        // hash|node-if-cache {node:hash,number:734,references:{self-parent-round-hash,external-round-hash}}
	graphPrefixSnapshot          = "SNAPSHOT"     //
	graphPrefixLink              = "LINK"         // self-external number
	graphPrefixTopology          = "TOPOLOGY"
	graphPrefixSnapTopology      = "SNAPTOPO"
	graphPrefixWorkLead          = "WORKPROPOSE"
	graphPrefixWorkSign          = "WORKVOTE"
	graphPrefixWorkOffset        = "WORKCHECKPOINT"
	graphPrefixWorkSnapshot      = "WORKSNAPSHOT"
	graphPrefixSpaceCheckpoint   = "SPACECHECKPOINT"
	graphPrefixSpaceQueue        = "SPACEQUEUE"
	graphPrefixAssetInfo         = "ASSETINFO"
	graphPrefixAssetTotal        = "ASSETTOTAL"
	graphPrefixCustodianUpdate   = "CUSTODIANUPDATE"
	graphPrefixConsensusSnapshot = "CONSENSUSSNAPSHOT"
)

func (s *BadgerStore) WriteConsensusSnapshot(snap *common.Snapshot, tx *common.VersionedTransaction) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	err := writeConsensusSnapshot(txn, snap, tx)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func writeConsensusSnapshot(txn *badger.Txn, snap *common.Snapshot, tx *common.VersionedTransaction) error {
	if snap.SoleTransaction() != tx.PayloadHash() {
		panic(snap.PayloadHash())
	}
	if len(tx.Inputs) == 1 && tx.Inputs[0].Mint != nil {
		logger.Printf("writeConsensusSnapshot(%s) => mint", snap.SoleTransaction())
	} else {
		out := tx.Outputs[0]
		switch out.Type {
		case common.OutputTypeNodePledge:
		case common.OutputTypeNodeCancel:
		case common.OutputTypeNodeAccept:
		case common.OutputTypeNodeRemove:
		case common.OutputTypeCustodianUpdateNodes:
		case common.OutputTypeCustodianSlashNodes:
		default:
			panic(out.Type)
		}
		logger.Printf("writeConsensusSnapshot(%s) => %d", snap.SoleTransaction(), out.Type)
	}

	isGenesis := len(tx.Inputs) == 1 && tx.Inputs[0].Genesis != nil
	last, referencedBy, err := readLastConsensusSnapshot(txn)
	if err != nil {
		return err
	}
	if !isGenesis && last.SoleTransaction() == tx.PayloadHash() {
		return nil
	}
	if referencedBy != nil {
		panic(snap.PayloadHash())
	}
	if !isGenesis {
		if last.SoleTransaction() != tx.References[0] {
			panic(snap.PayloadHash())
		}
		if last.Timestamp >= snap.Timestamp {
			panic(snap.PayloadHash())
		}
		key := graphConsensusSnapshotKey(last.Timestamp, last.PayloadHash())
		val := tx.PayloadHash()
		err := txn.Set(key, val[:])
		if err != nil {
			return err
		}
	}

	key := graphConsensusSnapshotKey(snap.Timestamp, snap.PayloadHash())
	return txn.Set(key, []byte{})
}

func (s *BadgerStore) ReadLastConsensusSnapshot() (*common.Snapshot, *crypto.Hash, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readLastConsensusSnapshot(txn)
}

func readLastConsensusSnapshot(txn *badger.Txn) (*common.Snapshot, *crypto.Hash, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphConsensusSnapshotKey(^uint64(0), crypto.Hash{}))
	if !it.ValidForPrefix([]byte(graphPrefixConsensusSnapshot)) {
		return nil, nil, nil
	}
	var h crypto.Hash
	key := it.Item().KeyCopy(nil)
	copy(h[:], key[len(graphPrefixConsensusSnapshot)+8:])
	snap, err := readSnapshotWithTopo(txn, h)
	if err != nil {
		return nil, nil, err
	}
	ts := binary.BigEndian.Uint64(key[len(graphPrefixConsensusSnapshot):])
	if snap.Timestamp != ts {
		panic(snap.PayloadHash())
	}

	val, err := it.Item().ValueCopy(nil)
	if err != nil {
		return nil, nil, err
	}
	if len(val) == 0 {
		return snap.Snapshot, nil, nil
	}
	copy(h[:], val)
	return snap.Snapshot, &h, nil
}

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
		s, err := common.UnmarshalVersionedSnapshot(v)
		if err != nil {
			return snapshots, err
		}
		s.Hash = s.PayloadHash()
		snapshots = append(snapshots, s)
	}

	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Timestamp < snapshots[j].Timestamp })
	return snapshots, nil
}

func (s *BadgerStore) WriteSnapshot(snap *common.SnapshotWithTopologicalOrder, signers []crypto.Hash) error {
	logger.Debugf("BadgerStore.WriteSnapshot(%v)", snap.Snapshot)
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
		ver, err := readTransaction(txn, snap.SoleTransaction())
		if err != nil {
			return err
		}
		if ver == nil {
			panic("snapshot transaction not exist")
		}
		key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.SoleTransaction())
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
		key = graphUniqueKey(snap.NodeId, snap.SoleTransaction())
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	ver, err := readTransaction(txn, snap.SoleTransaction())
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

	key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.SoleTransaction())
	val := snap.VersionedMarshal()
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	key = graphUniqueKey(snap.NodeId, snap.SoleTransaction())
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	return writeTopology(txn, snap)
}

func graphSnapshotKey(nodeId crypto.Hash, round uint64, hash crypto.Hash) []byte {
	key := append([]byte(graphPrefixSnapshot), nodeId[:]...)
	key = binary.BigEndian.AppendUint64(key, round)
	return append(key, hash[:]...)
}

func graphConsensusSnapshotKey(ts uint64, snap crypto.Hash) []byte {
	key := []byte(graphPrefixConsensusSnapshot)
	key = binary.BigEndian.AppendUint64(key, ts)
	return append(key, snap[:]...)
}
