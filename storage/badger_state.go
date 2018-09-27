package storage

import (
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) StateGet(key string, val interface{}) (bool, error) {
	txn := s.stateDB.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(key))
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return true, err
	}
	return true, msgpack.Unmarshal(ival, val)
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
