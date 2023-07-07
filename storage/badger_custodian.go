package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadCustodianAccount() (*common.Address, uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return s.readCustodianAccount(txn)
}

func (s *BadgerStore) readCustodianAccount(txn *badger.Txn) (*common.Address, uint64, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphCustodianAccountKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixCustodianAccount)) {
		key := it.Item().KeyCopy(nil)
		ts := graphCustodianAccountTimestamp(key)
		val, err := it.Item().ValueCopy(nil)
		if err != nil {
			return nil, 0, err
		}
		addr, err := common.NewAddressFromString(string(val))
		return &addr, ts, err
	}

	return nil, 0, nil
}

func (s *BadgerStore) writeCustodianNodes(txn *badger.Txn, snap *common.Snapshot, custodian *common.Address, nodes []*common.CustodianNode) error {
	old, ts, err := s.readCustodianAccount(txn)
	if err != nil {
		return err
	}
	switch {
	case ts == 0 && old != nil:
		panic(snap.Hash.String())
	case ts != 0 && old == nil:
		panic(snap.Hash.String())
	case old == nil && ts == 0:
	case ts > snap.Timestamp:
		panic(snap.Hash.String())
	case ts == snap.Timestamp && custodian.String() == old.String():
		return nil
	case ts == snap.Timestamp && custodian.String() != old.String():
		panic(snap.Hash.String())
	case ts < snap.Timestamp:
	}

	key := graphCustodianAccountKey(snap.Timestamp)
	err = txn.Set(key, []byte(custodian.String()))
	if err != nil {
		return err
	}
	panic(0)
}

func graphCustodianAccountKey(ts uint64) []byte {
	key := []byte(graphPrefixCustodianAccount)
	return binary.BigEndian.AppendUint64(key, ts)
}

func graphCustodianAccountTimestamp(key []byte) uint64 {
	ts := key[len(graphPrefixCustodianAccount):]
	return binary.BigEndian.Uint64(ts)
}
