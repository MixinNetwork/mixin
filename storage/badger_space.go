package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

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

		if space.Duration < uint64(config.CheckpointDuration) {
			panic(fmt.Errorf("WriteRoundSpaceAndState(%v) => invalid space", space))
		}
		key = append([]byte(graphPrefixSpaceQueue), space.NodeId[:]...)
		val = binary.BigEndian.AppendUint64(val, space.Duration)
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

func graphSpaceCheckpointKey(nodeId crypto.Hash) []byte {
	return append([]byte(graphPrefixSpaceCheckpoint), nodeId[:]...)
}
