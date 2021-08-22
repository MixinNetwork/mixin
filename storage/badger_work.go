package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

const DAY_U64 = uint64(time.Hour) * 24

func (s *BadgerStore) ReadWorkOffset(nodeId crypto.Hash) (uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	offKey := graphWorkOffsetKey(nodeId)
	return graphReadUint64(txn, offKey)
}

func (s *BadgerStore) ReadSnapshotWorksForNodeRound(nodeId crypto.Hash, round uint64) ([]*common.SnapshotWork, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphWorkSnaphotKey(nodeId, round, 0)
	prefix := key[:len(key)-8]

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()

	snapshots := make([]*common.SnapshotWork, 0)
	for it.Seek(key); it.Valid(); it.Next() {
		item := it.Item()
		var s common.SnapshotWork
		err := item.Value(func(v []byte) error {
			copy(s.Hash[:], v)
			for i := 32; i < len(v); i += 32 {
				var h crypto.Hash
				copy(h[:], v[i:])
				s.Signers = append(s.Signers, h)
			}
			return nil
		})
		if err != nil {
			return snapshots, err
		}
		ts := item.Key()[len(key)-8:]
		s.Timestamp = binary.BigEndian.Uint64(ts)
		snapshots = append(snapshots, &s)
	}

	return snapshots, nil
}

func (s *BadgerStore) ListWorkOffsets(cids []crypto.Hash) (map[crypto.Hash]uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	works := make(map[crypto.Hash]uint64)
	for _, id := range cids {
		key := graphWorkOffsetKey(id)
		off, err := graphReadUint64(txn, key)
		if err != nil {
			return nil, err
		}
		works[id] = off
	}

	return works, nil
}

func (s *BadgerStore) ListNodeWorks(cids []crypto.Hash, day uint32) (map[crypto.Hash][2]uint64, error) {
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

func (s *BadgerStore) WriteRoundWork(nodeId crypto.Hash, round uint64, snapshots []*common.SnapshotWork) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		offKey := graphWorkOffsetKey(nodeId)
		off, osm, err := graphReadWorkOffset(txn, offKey)
		if err != nil || off > round {
			return err
		}
		if round > off+1 {
			panic(fmt.Errorf("WriteRoundWork invalid offset %s %d %d", nodeId, off, round))
		}

		fresh := snapshots
		if round == off {
			fresh = make([]*common.SnapshotWork, 0)
			filter := make(map[crypto.Hash]bool)
			for _, ss := range snapshots {
				if !osm[ss.Hash] {
					fresh = append(fresh, ss)
				}
				filter[ss.Hash] = true
			}
			for id := range osm {
				if !filter[id] {
					panic(id)
				}
			}
		} else {
			err = removeSnapshotWorksForRound(txn, nodeId, off)
			if err != nil {
				return err
			}
		}

		err = graphWriteWorkOffset(txn, offKey, round, snapshots)
		if err != nil || len(fresh) == 0 {
			return err
		}
		if len(fresh[0].Signers) == 0 {
			return nil
		}

		day := uint32(fresh[0].Timestamp / DAY_U64)
		wm := make(map[crypto.Hash]uint64)
		for _, w := range fresh {
			if w.Timestamp == 0 {
				panic(w)
			}
			if uint32(w.Timestamp/DAY_U64) != day {
				panic(w)
			}
			if !w.Hash.HasValue() {
				panic(w)
			}
			for _, si := range w.Signers {
				wm[si] += 1
			}
		}
		if wm[nodeId] != uint64(len(fresh)) {
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

func writeSnapshotWork(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder, signers []crypto.Hash) error {
	key := graphWorkSnaphotKey(snap.NodeId, snap.RoundNumber, snap.Timestamp)
	val := make([]byte, (1+len(signers))*32)
	copy(val, snap.Hash[:])
	for i, h := range signers {
		copy(val[(i+1)*32:], h[:])
	}
	return txn.Set(key, val)
}

func removeSnapshotWorksForRound(txn *badger.Txn, nodeId crypto.Hash, round uint64) error {
	key := graphWorkSnaphotKey(nodeId, round, 0)
	prefix := key[:len(key)-8]

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Seek(key); it.Valid(); it.Next() {
		k := it.Item().KeyCopy(nil)
		err := txn.Delete(k)
		if err != nil {
			return err
		}
	}
	return nil
}

func graphWriteWorkOffset(txn *badger.Txn, key []byte, val uint64, snapshots []*common.SnapshotWork) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	for _, s := range snapshots {
		buf = append(buf, s.Hash[:]...)
	}
	return txn.Set(key, buf)
}

func graphReadWorkOffset(txn *badger.Txn, key []byte) (uint64, map[crypto.Hash]bool, error) {
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return 0, nil, err
	}

	round := binary.BigEndian.Uint64(ival[:8])
	snapshots := make(map[crypto.Hash]bool)
	for i, ival := 0, ival[8:]; i < len(ival)/32; i++ {
		var h crypto.Hash
		copy(h[:], ival[32*i:32*(i+1)])
		snapshots[h] = true
	}
	return round, snapshots, nil
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
	return binary.BigEndian.Uint64(ival[:8]), nil
}

func graphWorkOffsetKey(nodeId crypto.Hash) []byte {
	return append([]byte(graphPrefixWorkOffset), nodeId[:]...)
}

func graphWorkSignKey(nodeId crypto.Hash, day uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, day)
	key := append([]byte(graphPrefixWorkSign), nodeId[:]...)
	return append(key, buf...)
}

func graphWorkLeadKey(nodeId crypto.Hash, day uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, day)
	key := append([]byte(graphPrefixWorkLead), nodeId[:]...)
	return append(key, buf...)
}

func graphWorkSnaphotKey(nodeId crypto.Hash, round, ts uint64) []byte {
	buf := make([]byte, 8)
	key := append([]byte(graphPrefixWorkSnapshot), nodeId[:]...)
	binary.BigEndian.PutUint64(buf, round)
	key = append(key, buf...)
	binary.BigEndian.PutUint64(buf, ts)
	return append(key, buf...)
}
