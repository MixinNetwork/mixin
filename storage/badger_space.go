package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ListAggregatedRoundSpaceCheckpoints(cids []crypto.Hash) (map[crypto.Hash]*common.RoundSpace, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	spaces := make(map[crypto.Hash]*common.RoundSpace)
	for _, id := range cids {
		batch, round, err := s.ReadRoundSpaceCheckpoint(id)
		if err != nil {
			return nil, err
		}
		spaces[id] = &common.RoundSpace{
			NodeId: id,
			Batch:  batch,
			Round:  round,
		}
	}
	return spaces, nil
}

func (s *BadgerStore) ReadNodeRoundSpacesForBatch(nodeId crypto.Hash, batch uint64) ([]*common.RoundSpace, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	var spaces []*common.RoundSpace
	key := graphSpaceQueueKey(nodeId, batch, 0)
	prefix := key[:len(key)-8]

	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Seek(key); it.Valid(); it.Next() {
		item := it.Item()
		val, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		if bytes.Compare(nodeId[:], item.Key()[:32]) != 0 {
			panic(nodeId)
		}
		if binary.BigEndian.Uint64(item.Key()[32:40]) != batch {
			panic(batch)
		}
		space := &common.RoundSpace{
			NodeId:   nodeId,
			Batch:    batch,
			Round:    binary.BigEndian.Uint64(item.Key()[40:]),
			Duration: binary.BigEndian.Uint64(val),
		}
		spaces = append(spaces, space)
	}

	return spaces, nil
}

func (s *BadgerStore) ReadRoundSpaceCheckpoint(nodeId crypto.Hash) (uint64, uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readRoundSpaceCheckpoint(txn, nodeId)
}

func (s *BadgerStore) WriteRoundSpaceAndState(space *common.RoundSpace) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		ob, or, err := readRoundSpaceCheckpoint(txn, space.NodeId)
		if err != nil {
			return err
		}
		if ob > space.Batch || or > space.Round {
			panic(fmt.Errorf("WriteRoundSpaceAndState(%v) => invalid round %d:%d", space, ob, or))
		}

		key := graphSpaceCheckpointKey(space.NodeId)
		val := binary.BigEndian.AppendUint64(nil, space.Batch)
		val = binary.BigEndian.AppendUint64(val, space.Round)
		err = txn.Set(key, val)
		if err != nil || space.Duration == 0 {
			return err
		}
		if space.Round == 0 {
			panic(fmt.Errorf("WriteRoundSpaceAndState(%v) => first accepted round", space))
		}

		if space.Duration < uint64(config.CheckpointDuration) {
			panic(fmt.Errorf("WriteRoundSpaceAndState(%v) => invalid space", space))
		}
		key = graphSpaceQueueKey(space.NodeId, space.Batch, space.Round)
		val = binary.BigEndian.AppendUint64(nil, space.Duration)
		return txn.Set(key, val)
	})

}

func readRoundSpaceCheckpoint(txn *badger.Txn, nodeId crypto.Hash) (uint64, uint64, error) {
	key := graphSpaceCheckpointKey(nodeId)
	item, err := txn.Get(key)

	if err == badger.ErrKeyNotFound {
		return 0, 0, nil
	} else if err != nil {
		return 0, 0, err
	}

	val, err := item.ValueCopy(nil)
	if err != nil {
		return 0, 0, err
	}
	batch := binary.BigEndian.Uint64(val[:8])
	round := binary.BigEndian.Uint64(val[8:16])
	return batch, round, nil
}

func graphSpaceQueueKey(nodeId crypto.Hash, batch, round uint64) []byte {
	key := append([]byte(graphPrefixSpaceQueue), nodeId[:]...)
	key = binary.BigEndian.AppendUint64(key, batch)
	return binary.BigEndian.AppendUint64(key, round)
}

func graphSpaceCheckpointKey(nodeId crypto.Hash) []byte {
	return append([]byte(graphPrefixSpaceCheckpoint), nodeId[:]...)
}
