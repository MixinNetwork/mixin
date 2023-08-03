package storage

import (
	"encoding/binary"
	"encoding/hex"
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

	var curs []*common.CustodianUpdateRequest
	it.Seek(graphCustodianUpdateKey(0))
	for ; it.ValidForPrefix([]byte(graphPrefixCustodianUpdate)); it.Next() {
		cur, err := parseCustodianUpdateItem(it)
		if err != nil {
			return nil, err
		}
		curs = append(curs, cur)
	}
	return curs, nil
}

func (s *BadgerStore) ReadCustodian(ts uint64) (*common.CustodianUpdateRequest, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readCustodianAccount(txn, ts)
}

func parseCustodianNodes(val []byte) ([]*common.CustodianNode, error) {
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

func readCustodianAccount(txn *badger.Txn, ts uint64) (*common.CustodianUpdateRequest, error) {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = []byte(graphPrefixCustodianUpdate)
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphCustodianUpdateKey(ts))
	if it.ValidForPrefix([]byte(graphPrefixCustodianUpdate)) {
		return parseCustodianUpdateItem(it)
	}

	return nil, nil
}

func parseCustodianUpdateItem(it *badger.Iterator) (*common.CustodianUpdateRequest, error) {
	key := it.Item().KeyCopy(nil)
	ts := graphCustodianAccountTimestamp(key)
	val, err := it.Item().ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	if len(val) < 97 {
		panic(len(val))
	}

	nodes, err := parseCustodianNodes(val[96:])
	if err != nil {
		return nil, err
	}

	var hash crypto.Hash
	copy(hash[:], val[:32])
	var account common.Address
	copy(account.PublicSpendKey[:], val[32:64])
	copy(account.PublicViewKey[:], val[64:96])
	return &common.CustodianUpdateRequest{
		Custodian:   &account,
		Nodes:       nodes,
		Transaction: hash,
		Timestamp:   ts,
	}, nil
}

func writeCustodianNodes(txn *badger.Txn, snapTime uint64, utxo *common.UTXOWithLock, extra []byte) error {
	custodian, nodes, _, err := common.ParseCustodianUpdateNodesExtra(extra)
	if err != nil {
		panic(fmt.Errorf("common.ParseCustodianUpdateNodesExtra(%x) => %v", extra, err))
	}
	cur, err := readCustodianAccount(txn, snapTime)
	if err != nil {
		return err
	}
	switch {
	case cur == nil:
	case cur.Timestamp > snapTime:
		panic(utxo.Hash.String())
	case cur.Timestamp == snapTime && custodian.String() == cur.Custodian.String():
		return nil
	case cur.Timestamp == snapTime && custodian.String() != cur.Custodian.String():
		panic(utxo.Hash.String())
	case cur.Timestamp < snapTime:
	}

	key := graphCustodianUpdateKey(snapTime)
	val := append(utxo.Hash[:], custodian.PublicSpendKey[:]...)
	val = append(val, custodian.PublicViewKey[:]...)
	if len(nodes) > 50 {
		panic(len(nodes))
	}
	val = append(val, byte(len(nodes)))
	for _, n := range nodes {
		val = append(val, n.Extra...)
	}

	return txn.Set(key, val)
}

func graphCustodianUpdateKey(ts uint64) []byte {
	key := []byte(graphPrefixCustodianUpdate)
	return binary.BigEndian.AppendUint64(key, ts)
}

func graphCustodianAccountTimestamp(key []byte) uint64 {
	ts := key[len(graphPrefixCustodianUpdate):]
	return binary.BigEndian.Uint64(ts)
}
