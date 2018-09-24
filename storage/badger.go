package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

type BadgerStore struct {
	snapshotsDB *badger.DB
	mempoolDB   *badger.DB
	queueDB     *badger.DB
	stateDB     *badger.DB
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	snapshotsDB, err := openDB(dir+"/snapshots", false)
	if err != nil {
		return nil, err
	}
	mempoolDB, err := openDB(dir+"/mempool", false)
	if err != nil {
		return nil, err
	}
	queueDB, err := openDB(dir+"/queue", false)
	if err != nil {
		return nil, err
	}
	stateDB, err := openDB(dir+"/state", true)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{
		snapshotsDB: snapshotsDB,
		mempoolDB:   mempoolDB,
		queueDB:     queueDB,
		stateDB:     stateDB,
	}, nil
}

func (s *BadgerStore) StateGet(key string, val interface{}) error {
	return s.stateDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return ErrorNotFound
		}
		if err != nil {
			return err
		}
		ival, err := item.Value()
		if err != nil {
			return err
		}
		return msgpack.Unmarshal(ival, val)
	})
}

func (s *BadgerStore) StateSet(key string, val interface{}) error {
	return s.stateDB.Update(func(txn *badger.Txn) error {
		ival, err := msgpack.Marshal(val)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), ival)
	})
}

func (s *BadgerStore) SnapshotsLoadGenesis(snapshots []*common.Snapshot) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		if checkGenesisLoad(txn) {
			return nil
		}

		for _, s := range snapshots {
			err := saveSnapshot(txn, s, true)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *BadgerStore) SnapshotsGetUTXO(hash crypto.Hash, index int) (*common.UTXO, error) {
	var utxo common.UTXO
	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		key := utxoKey(hash, index)
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return ErrorNotFound
		}
		if err != nil {
			return err
		}
		ival, err := item.Value()
		if err != nil {
			return err
		}
		return msgpack.Unmarshal(ival, &utxo)
	})
	return &utxo, err
}

func (s *BadgerStore) SnapshotsGetKey(key crypto.Key) (bool, error) {
	var exist bool
	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		key := addressKey(key)
		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		exist = true
		return nil
	})
	return exist, err
}

func (s *BadgerStore) SnapshotsListSince(offset, count uint64) ([]*common.SnapshotWithHash, error) {
	snapshots := make([]*common.SnapshotWithHash, 0)

	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(snapshotKey(offset)); it.ValidForPrefix([]byte("SNAPSHOT")) && uint64(len(snapshots)) < count; it.Next() {
			item := it.Item()
			v, err := item.Value()
			if err != nil {
				return err
			}
			var s common.SnapshotWithHash
			err = msgpack.Unmarshal(v, &s)
			if err != nil {
				return err
			}
			s.Hash = s.Transaction.Hash()
			snapshots = append(snapshots, &s)
		}
		return nil
	})
	return snapshots, err
}

func (s *BadgerStore) SnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	snapshots := make([]*common.Snapshot, 0)

	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		key := nodeKey(nodeIdWithNetwork, round, 0)
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

func checkGenesisLoad(txn *badger.Txn) bool {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Rewind()
	return it.Valid()
}

// snapshotsDB: add snapshot should use UTXO, KEY, SNAPSHOT, NODE prefix to add different data in a single badger txn
// UTXO is the unspent output database
// KEY is unqiue globally, may be combined this with the UTXO? no, becase multisig
// SNAPSHOT is the sorted snapshots list, I don't understand if we manage an atomic single list, what's the point of DAG
// NODE is the node seperated snapshots list for each node

// SNAPSHOT sorted snapshots list is only for reference only, not for consensus.

func saveSnapshot(txn *badger.Txn, snapshot *common.Snapshot, genesis bool) error {
	outputs, err := snapshot.UTXOs()
	if err != nil {
		return err
	}

	for _, in := range snapshot.Transaction.Inputs {
		if genesis && in.Hash.String() == (crypto.Hash{}).String() {
			continue
		}
		key := utxoKey(in.Hash, in.Index)
		_, err := txn.Get(key)
		if err != nil {
			return err
		}
		err = txn.Delete(key)
		if err != nil {
			return err
		}
	}

	for _, utxo := range outputs {
		for _, k := range utxo.Keys {
			key := addressKey(k)
			_, err := txn.Get(key)
			if err == nil {
				return fmt.Errorf("duplicated key %s", k.String())
			} else if err != badger.ErrKeyNotFound {
				return err
			}
			err = txn.Set(key, []byte{0})
			if err != nil {
				return err
			}
		}
		key := utxoKey(utxo.Hash, utxo.Index)
		_, err := txn.Get(key)
		if err == nil {
			return fmt.Errorf("duplicated utxo %s", utxo.Hash.String())
		} else if err != badger.ErrKeyNotFound {
			return err
		}
		val := common.MsgpackMarshalPanic(utxo)
		err = txn.Set(key, val)
		if err != nil {
			return err
		}
	}

	key := nodeKey(snapshot.NodeId, snapshot.RoundNumber, snapshot.Timestamp)
	_, err = txn.Get(key)
	if err == nil {
		return fmt.Errorf("duplicated snapshot %s", snapshot.Transaction.Hash().String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	val := common.MsgpackMarshalPanic(snapshot)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	return txn.Set(snapshotKey(uint64(time.Now().UnixNano())), val)
}

func addressKey(k crypto.Key) []byte {
	return append([]byte("KEY"), k[:]...)
}

func snapshotKey(ts uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, ts)
	return append([]byte("SNAPSHOT"), buf...)
}

func nodeKey(nodeIdWithNetwork crypto.Hash, round, ts uint64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, round)
	binary.BigEndian.PutUint64(buf, ts)
	key := append([]byte("NODE"), nodeIdWithNetwork[:]...)
	return append(key, buf...)
}

func utxoKey(hash crypto.Hash, index int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte("UTXO"), hash[:]...)
	return append(key, buf[:size]...)
}

func (s *BadgerStore) QueueAdd(tx *common.SignedTransaction) error {
	return s.queueDB.Update(func(txn *badger.Txn) error {
		ival, err := msgpack.Marshal(tx)
		if err != nil {
			return err
		}
		key := queueTxKey(uint64(time.Now().UnixNano()))
		return txn.Set([]byte(key), ival)
	})
}

func (s *BadgerStore) QueuePoll(offset uint64, hook func(k uint64, v []byte) error) error {
	return s.queueDB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(queueTxKey(offset)); it.ValidForPrefix([]byte("TX")); it.Next() {
			item := it.Item()
			k := item.Key()
			v, err := item.Value()
			if err != nil {
				return err
			}
			err = hook(binary.BigEndian.Uint64(k[2:]), v)
			if err != nil {
				return err
			}
			err = txn.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func queueTxKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte("TX"), buf...)
}

func openDB(dir string, sync bool) (*badger.DB, error) {
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	opts.SyncWrites = sync
	opts.NumVersionsToKeep = 1
	return badger.Open(opts)
}
