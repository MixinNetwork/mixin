package storage

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadCustodian(ts uint64) (*common.Address, []*common.CustodianNode, uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	account, nodesVal, at, err := s.readCustodianAccount(txn, ts)
	if err != nil || account == nil {
		return nil, nil, 0, err
	}
	nodes, err := s.parseCustodianNodes(nodesVal)
	if err != nil {
		return nil, nil, 0, err
	}
	return account, nodes, at, nil
}

func (s *BadgerStore) parseCustodianNodes(val []byte) ([]*common.CustodianNode, error) {
	count := int(val[0])
	size := len(val[1:]) / count
	if size*count != len(val)-1 {
		panic(hex.EncodeToString(val))
	}
	nodes := make([]*common.CustodianNode, count)
	for i := 0; i < int(val[0]); i++ {
		extra := val[1+i*size : 1+(i+1)*size]
		node, err := common.ParseCustodianNode(extra)
		if err != nil {
			return nil, err
		}
		nodes[i] = node
	}
	return nodes, nil
}

func (s *BadgerStore) readCustodianAccount(txn *badger.Txn, ts uint64) (*common.Address, []byte, uint64, error) {
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
			return nil, nil, 0, err
		}
		if len(val) < 65 {
			panic(len(val))
		}
		var account common.Address
		copy(account.PublicSpendKey[:], val[:32])
		copy(account.PublicSpendKey[:], val[32:64])
		return &account, val[64:], ts, nil
	}

	return nil, nil, 0, nil
}

func (s *BadgerStore) writeCustodianNodes(txn *badger.Txn, snap *common.Snapshot, custodian *common.Address, nodes []*common.CustodianNode) error {
	old, _, ts, err := s.readCustodianAccount(txn, snap.Timestamp)
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
	val := append(custodian.PublicSpendKey[:], custodian.PublicViewKey[:]...)
	if len(nodes) > 50 {
		panic(len(nodes))
	}
	val = append(val, byte(len(nodes)))
	for _, n := range nodes {
		val = append(val, n.Extra...)
	}

	err = txn.Set(key, val)
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
