package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

const (
	BlockNumberEmpty = ^uint64(0)

	sequencerPrefixTopologyOffset = "SEQUENCER:TOPOLOGY"
	sequencerPrefixBlock          = "SEQUENCER:BLOCK"
	sequencerPrefixSnapshotBlock  = "SEQUENCER:SNAPSHOT"
)

func (s *BadgerStore) ReadSequencedTopology() (uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(sequencerPrefixTopologyOffset))
	if err == badger.ErrKeyNotFound {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(val), nil
}

func (s *BadgerStore) ReadLastBlock() (*common.Block, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(sequencerBlockKey(BlockNumberEmpty))
	if !it.ValidForPrefix([]byte(sequencerPrefixBlock)) {
		return nil, nil
	}
	val, err := it.Item().ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	return common.UnmarshalBlock(val)
}

func (s *BadgerStore) ReadBlockWithTransactions(number uint64) (*common.BlockWithTransactions, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := sequencerBlockKey(number)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	block, err := common.UnmarshalBlock(val)
	if err != nil {
		return nil, err
	}
	bws := &common.BlockWithTransactions{
		Block:        *block,
		Snapshots:    make(map[crypto.Hash]*common.Snapshot, len(block.Snapshots)),
		Transactions: make(map[crypto.Hash]*common.VersionedTransaction),
	}
	for _, h := range block.Snapshots {
		s, err := readSnapshotWithTopo(txn, h)
		if err != nil {
			return nil, err
		}
		bws.Snapshots[s.Hash] = s.Snapshot
		for _, h := range s.Transactions {
			t, err := readTransaction(txn, h)
			if err != nil {
				return nil, err
			}
			bws.Transactions[h] = t
		}
	}
	return bws, nil
}

func (s *BadgerStore) WriteBlock(b *common.Block) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	for i, h := range b.Snapshots {
		key := sequencerSnapshotBlockKey(h)
		val := binary.BigEndian.AppendUint64(nil, b.Number)
		val = binary.BigEndian.AppendUint64(val, b.Number+uint64(i))
		err := txn.Set(key, val)
		if err != nil {
			return err
		}
	}

	key := sequencerBlockKey(b.Number)
	val := b.Marshal()
	err := txn.Set(key, val)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (s *BadgerStore) CheckSnapshotsSequencedIn(snapshots []crypto.Hash) (map[crypto.Hash]uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	ssi := make(map[crypto.Hash]uint64)
	for _, h := range snapshots {
		b, _, err := readSnapshotSequence(txn, h)
		if err != nil {
			return ssi, err
		}
		if b != BlockNumberEmpty {
			ssi[h] = b
		}
	}
	return ssi, nil
}

func (s *BadgerStore) ReadUnsequencedSnapshotsSinceTopology(nodeId crypto.Hash, offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, map[crypto.Hash]*common.VersionedTransaction, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	snapshots, err := readSnapshotsSinceTopology(txn, offset, count, nodeId)
	if err != nil {
		return nil, nil, err
	}

	transactions := make(map[crypto.Hash]*common.VersionedTransaction)
	for _, s := range snapshots {
		h := s.SoleTransaction()
		tx, err := readTransaction(txn, h)
		if err != nil {
			return nil, nil, err
		}
		transactions[h] = tx
	}
	return snapshots, transactions, nil
}

func readSnapshotSequence(txn *badger.Txn, hash crypto.Hash) (uint64, uint64, error) {
	key := sequencerSnapshotBlockKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return BlockNumberEmpty, BlockNumberEmpty, nil
	} else if err != nil {
		return BlockNumberEmpty, BlockNumberEmpty, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return BlockNumberEmpty, BlockNumberEmpty, err
	}
	number := binary.BigEndian.Uint64(val[:8])
	sequence := binary.BigEndian.Uint64(val[8:])
	return number, sequence, nil
}

func sequencerBlockKey(num uint64) []byte {
	key := []byte(sequencerPrefixBlock)
	return binary.BigEndian.AppendUint64(key, num)
}

func sequencerSnapshotBlockKey(hash crypto.Hash) []byte {
	key := []byte(sequencerPrefixSnapshotBlock)
	return append(key, hash[:]...)
}
