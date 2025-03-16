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
	sequencerPrefixBlockNumber    = "SEQUENCER:BLOCK:NUMBER"
	sequencerPrefixBlockHash      = "SEQUENCER:BLOCK:HASH"
	sequencerPrefixSnapshotBlock  = "SEQUENCER:SNAPSHOT"
)

func (s *BadgerStore) ReadSequencedTopology() (uint64, error) {
	txn := s.sequencerDB.NewTransaction(false)
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
	txn := s.sequencerDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(sequencerBlockNumberKey(BlockNumberEmpty))
	if !it.ValidForPrefix([]byte(sequencerPrefixBlockNumber)) {
		return nil, nil
	}
	val, err := it.Item().ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	return common.UnmarshalBlock(val)
}

func (s *BadgerStore) ReadBlockByHash(hash crypto.Hash) (*common.BlockWithTransactions, error) {
	number, err := s.ReadBlockNumber(hash)
	if err != nil {
		return nil, err
	}
	return s.ReadBlockWithTransactions(number)
}

func (s *BadgerStore) ReadBlockWithTransactions(number uint64) (*common.BlockWithTransactions, error) {
	block, err := s.ReadBlock(number)
	if err != nil {
		return nil, err
	}
	snapshots, transactions, err := s.ReadSnapshots(block.Snapshots)
	if err != nil {
		return nil, err
	}
	bws := &common.BlockWithTransactions{
		Block:        *block,
		Snapshots:    snapshots,
		Transactions: transactions,
	}
	return bws, nil
}

func (s *BadgerStore) ReadBlock(number uint64) (*common.Block, error) {
	txn := s.sequencerDB.NewTransaction(false)
	defer txn.Discard()

	key := sequencerBlockNumberKey(number)
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
	return common.UnmarshalBlock(val)
}

func (s *BadgerStore) WriteBlock(b *common.Block, topology uint64) error {
	txn := s.sequencerDB.NewTransaction(true)
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

	key := sequencerBlockNumberKey(b.Number)
	val := b.Marshal()
	err := txn.Set(key, val)
	if err != nil {
		return err
	}
	key = sequencerBlockHashKey(b.PayloadHash())
	val = binary.BigEndian.AppendUint64(nil, b.Number)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}
	if topology > 0 {
		key := []byte(sequencerPrefixTopologyOffset)
		val := binary.BigEndian.AppendUint64(nil, topology)
		err := txn.Set(key, val)
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *BadgerStore) ReadBlockNumber(hash crypto.Hash) (uint64, error) {
	txn := s.sequencerDB.NewTransaction(false)
	defer txn.Discard()

	key := sequencerBlockHashKey(hash)
	item, err := txn.Get(key)
	if err != nil {
		return 0, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(val), nil
}

func (s *BadgerStore) CheckSnapshotsSequencedIn(snapshots []crypto.Hash) (map[crypto.Hash]uint64, error) {
	txn := s.sequencerDB.NewTransaction(false)
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
	var unsequenced []*common.SnapshotWithTopologicalOrder
	for uint64(len(unsequenced)) < count {
		snapshots, err := s.ReadSnapshotsSinceTopology(offset, count)
		if err != nil {
			return nil, nil, err
		} else if len(snapshots) == 0 {
			break
		}
		offset = offset + count
		var candis []*common.SnapshotWithTopologicalOrder
		var candiHashes []crypto.Hash
		for _, s := range snapshots {
			if s.NodeId == nodeId {
				continue
			}
			candis = append(candis, s)
			candiHashes = append(candiHashes, s.Hash)
		}
		if len(candis) == 0 {
			continue
		}
		ssi, err := s.CheckSnapshotsSequencedIn(candiHashes)
		if err != nil {
			return nil, nil, err
		}
		for _, h := range candis {
			if _, found := ssi[h.Hash]; found {
				continue
			}
			unsequenced = append(unsequenced, h)
		}
	}

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	transactions := make(map[crypto.Hash]*common.VersionedTransaction)
	for _, s := range unsequenced {
		h := s.SoleTransaction()
		tx, err := readTransaction(txn, h)
		if err != nil {
			return nil, nil, err
		}
		transactions[h] = tx
	}
	return unsequenced, transactions, nil
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

func sequencerBlockNumberKey(num uint64) []byte {
	key := []byte(sequencerPrefixBlockNumber)
	return binary.BigEndian.AppendUint64(key, num)
}

func sequencerBlockHashKey(hash crypto.Hash) []byte {
	key := []byte(sequencerPrefixBlockHash)
	return append(key, hash[:]...)
}

func sequencerSnapshotBlockKey(hash crypto.Hash) []byte {
	key := []byte(sequencerPrefixSnapshotBlock)
	return append(key, hash[:]...)
}
