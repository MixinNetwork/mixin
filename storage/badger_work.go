package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v2"
)

const DAY_U64 = uint64(time.Hour) * 24

func (s *BadgerStore) ReadWorkOffset(nodeId crypto.Hash) (uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	offKey := graphWorkOffsetKey(nodeId)
	return graphReadUint64(txn, offKey)
}

func (s *BadgerStore) ListNodeWorks(cids []crypto.Hash, day uint64) (map[crypto.Hash][2]uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	works := make(map[crypto.Hash][2]uint64)
	for _, id := range cids {
		lk := graphWorkLeadKey(id, day)
		lw, err := graphReadUint64(txn, lk)
		if err != nil {
			return nil, err
		}
		sk := graphWorkSignKey(id, day)
		sw, err := graphReadUint64(txn, sk)
		if err != nil {
			return nil, err
		}
		works[id] = [2]uint64{lw, sw}
	}

	return works, nil
}

func (s *BadgerStore) WriteRoundWork(nodeId crypto.Hash, round, day uint64, snapshots []*common.SnapshotWithTopologicalOrder) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		offKey := graphWorkOffsetKey(nodeId)
		oldOffset, err := graphReadUint64(txn, offKey)
		if err != nil || oldOffset >= round {
			return err
		}
		if round != oldOffset+1 {
			panic(nodeId)
		}
		err = graphWriteUint64(txn, offKey, round)
		if err != nil {
			return err
		}

		wm := make(map[crypto.Hash]uint64)
		for _, w := range snapshots {
			if w.NodeId != nodeId {
				panic(w)
			}
			if w.RoundNumber != round {
				panic(w)
			}
			if w.Timestamp/DAY_U64 != day {
				panic(w)
			}
			for _, si := range w.Signers {
				wm[si] += 1
			}
		}
		if wm[nodeId] != uint64(len(snapshots)) {
			panic(nodeId)
		}

		for ni, wn := range wm {
			if ni == nodeId {
				continue
			}
			signKey := graphWorkSignKey(ni, day)
			os, err := graphReadUint64(txn, signKey)
			if err != nil {
				return err
			}
			err = graphWriteUint64(txn, signKey, os+wn)
			if err != nil {
				return err
			}
		}

		leadKey := graphWorkLeadKey(nodeId, day)
		ol, err := graphReadUint64(txn, leadKey)
		if err != nil {
			return err
		}
		return graphWriteUint64(txn, leadKey, ol+wm[nodeId])
	})
}

func graphWriteUint64(txn *badger.Txn, key []byte, val uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return txn.Set(key, buf)
}

func graphReadUint64(txn *badger.Txn, key []byte) (uint64, error) {
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

func graphWorkOffsetKey(nodeId crypto.Hash) []byte {
	return append([]byte(graphPrefixWorkOffset), nodeId[:]...)
}

func graphWorkSignKey(nodeId crypto.Hash, day uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, day)
	key := append([]byte(graphPrefixWorkSign), nodeId[:]...)
	return append(key, buf...)
}

func graphWorkLeadKey(nodeId crypto.Hash, day uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, day)
	key := append([]byte(graphPrefixWorkLead), nodeId[:]...)
	return append(key, buf...)
}
