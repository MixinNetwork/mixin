package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v2"
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
	return true, common.MsgpackUnmarshal(ival, val)
}

func (s *BadgerStore) StateSet(key string, val interface{}) error {
	return s.stateDB.Update(func(txn *badger.Txn) error {
		ival := common.MsgpackMarshalPanic(val)
		return txn.Set([]byte(key), ival)
	})
}
