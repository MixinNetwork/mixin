package storage

import (
	"encoding/binary"
	"fmt"
	"sync"

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
		cur, err := parseCustodianUpdateItem(txn, it, genesis, &s.custodians)
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

	return readCustodianAccount(txn, ts, &s.custodians)
}

func readCustodianAccount(txn *badger.Txn, ts uint64, cache *sync.Map) (*common.CustodianUpdateRequest, error) {
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
		key := it.Item().Key()
		if graphCustodianAccountTimestamp(key) > ts {
			break
		}
		cur, err := parseCustodianUpdateItem(txn, it, genesis, cache)
		if err != nil {
			return nil, err
		}
		found = cur
		genesis = false
	}

	return found, nil
}

type custodianCacheKey struct {
	transaction crypto.Hash
	genesis     bool
}

func parseCustodianUpdateItem(txn *badger.Txn, it *badger.Iterator, genesis bool, cache *sync.Map) (*common.CustodianUpdateRequest, error) {
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
	cacheKey := custodianCacheKey{transaction: hash, genesis: genesis}
	if cache != nil {
		if cached, ok := cache.Load(cacheKey); ok {
			return cloneCustodianUpdate(cached.(*common.CustodianUpdateRequest), hash, ts), nil
		}
	}
	tx, err := readTransaction(txn, hash)
	if err != nil {
		return nil, err
	}
	cur, err := common.ParseCustodianUpdateNodesExtra(tx.Extra, genesis)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cached, _ := cache.LoadOrStore(cacheKey, cur)
		cur = cached.(*common.CustodianUpdateRequest)
	}
	return cloneCustodianUpdate(cur, hash, ts), nil
}

func cloneCustodianUpdate(cur *common.CustodianUpdateRequest, hash crypto.Hash, ts uint64) *common.CustodianUpdateRequest {
	cloned := *cur
	cloned.Transaction = hash
	cloned.Timestamp = ts
	if cur.Custodian != nil {
		custodian := *cur.Custodian
		cloned.Custodian = &custodian
	}
	if cur.Signature != nil {
		signature := *cur.Signature
		cloned.Signature = &signature
	}
	cloned.Nodes = make([]*common.CustodianNode, len(cur.Nodes))
	for i, node := range cur.Nodes {
		clonedNode := *node
		clonedNode.Extra = append([]byte(nil), node.Extra...)
		cloned.Nodes[i] = &clonedNode
	}
	return &cloned
}

func writeCustodianNodes(txn *badger.Txn, snapTime uint64, utxo *common.UTXOWithLock, extra []byte, genesis bool) error {
	now, err := common.ParseCustodianUpdateNodesExtra(extra, genesis)
	if err != nil {
		panic(fmt.Errorf("common.ParseCustodianUpdateNodesExtra(%x, %t) => %v", extra, genesis, err))
	}
	if len(now.Nodes) > 50 {
		panic(len(now.Nodes))
	}
	prev, err := readCustodianAccount(txn, snapTime, nil)
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
