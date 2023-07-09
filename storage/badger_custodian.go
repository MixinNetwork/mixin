package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadCustodian(ts uint64) (*common.Address, []*common.CustodianNode, uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	account, at, err := s.readCustodianAccount(txn, ts)
	if err != nil {
		return nil, nil, 0, err
	}
	nodes, err := s.readCustodianNodes(txn, ts)
	if err != nil {
		return nil, nil, 0, err
	}
	return account, nodes, at, nil
}

func (s *BadgerStore) readCustodianNodes(txn *badger.Txn, ts uint64) ([]*common.CustodianNode, error) {
	return nil, nil
}

func (s *BadgerStore) readCustodianAccount(txn *badger.Txn, ts uint64) (*common.Address, uint64, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphCustodianUpdateKey(ts))
	if it.ValidForPrefix([]byte(graphPrefixCustodianUpdate)) {
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
	old, ts, err := s.readCustodianAccount(txn, snap.Timestamp)
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

	key := graphCustodianUpdateKey(snap.Timestamp)
	err = txn.Set(key, []byte(custodian.String()))
	if err != nil {
		return err
	}
	panic(0)
}

func graphCustodianUpdateKey(ts uint64) []byte {
	key := []byte(graphPrefixCustodianUpdate)
	return binary.BigEndian.AppendUint64(key, ts)
}

func graphCustodianAccountTimestamp(key []byte) uint64 {
	ts := key[len(graphPrefixCustodianUpdate):]
	return binary.BigEndian.Uint64(ts)
}
