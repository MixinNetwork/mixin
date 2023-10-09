package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ListCustodianUpdates() ([]*common.CustodianUpdateRequest, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = []byte(graphPrefixCustodianUpdate)
	opts.Reverse = false

	it := txn.NewIterator(opts)
	defer it.Close()

	genesis := true
	var curs []*common.CustodianUpdateRequest
	it.Seek(graphCustodianUpdateKey(0))
	for ; it.ValidForPrefix([]byte(graphPrefixCustodianUpdate)); it.Next() {
		cur, err := parseCustodianUpdateItem(txn, it, genesis)
		if err != nil {
			return nil, err
		}
		curs = append(curs, cur)
		genesis = false
	}
	return curs, nil
}

func (s *BadgerStore) ReadCustodian(ts uint64) (*common.CustodianUpdateRequest, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readCustodianAccount(txn, ts)
}

func readCustodianAccount(txn *badger.Txn, ts uint64) (*common.CustodianUpdateRequest, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = []byte(graphPrefixCustodianUpdate)
	opts.Reverse = false

	it := txn.NewIterator(opts)
	defer it.Close()

	genesis := true
	var found *common.CustodianUpdateRequest
	it.Seek(graphCustodianUpdateKey(0))
	for ; it.ValidForPrefix([]byte(graphPrefixCustodianUpdate)); it.Next() {
		cur, err := parseCustodianUpdateItem(txn, it, genesis)
		if err != nil {
			return nil, err
		}
		if cur.Timestamp > ts {
			break
		}
		found = cur
		genesis = false
	}

	return found, nil
}

func parseCustodianUpdateItem(txn *badger.Txn, it *badger.Iterator, genesis bool) (*common.CustodianUpdateRequest, error) {
	key := it.Item().KeyCopy(nil)
	ts := graphCustodianAccountTimestamp(key)
	val, err := it.Item().ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	if len(val) != 32 {
		panic(len(val))
	}

	var hash crypto.Hash
	copy(hash[:], val)
	tx, err := readTransaction(txn, hash)
	if err != nil {
		return nil, err
	}
	cur, err := common.ParseCustodianUpdateNodesExtra(tx.Extra, genesis)
	if err != nil {
		return nil, err
	}
	cur.Transaction = hash
	cur.Timestamp = ts
	return cur, nil
}

func writeCustodianNodes(txn *badger.Txn, snapTime uint64, utxo *common.UTXOWithLock, extra []byte, genesis bool) error {
	now, err := common.ParseCustodianUpdateNodesExtra(extra, genesis)
	if err != nil {
		panic(fmt.Errorf("common.ParseCustodianUpdateNodesExtra(%x, %t) => %v", extra, genesis, err))
	}
	if len(now.Nodes) > 50 {
		panic(len(now.Nodes))
	}
	prev, err := readCustodianAccount(txn, snapTime)
	if err != nil {
		return err
	}
	switch {
	case prev == nil:
	case prev.Timestamp > snapTime:
		panic(utxo.Hash.String())
	case prev.Timestamp == snapTime && now.Custodian.String() == prev.Custodian.String():
		return nil
	case prev.Timestamp == snapTime && now.Custodian.String() != prev.Custodian.String():
		panic(utxo.Hash.String())
	case prev.Timestamp < snapTime:
	}

	key := graphCustodianUpdateKey(snapTime)
	return txn.Set(key, utxo.Hash[:])
}

func graphCustodianUpdateKey(ts uint64) []byte {
	key := []byte(graphPrefixCustodianUpdate)
	return binary.BigEndian.AppendUint64(key, ts)
}

func graphCustodianAccountTimestamp(key []byte) uint64 {
	ts := key[len(graphPrefixCustodianUpdate):]
	return binary.BigEndian.Uint64(ts)
}
