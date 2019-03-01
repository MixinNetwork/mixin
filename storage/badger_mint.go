package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) ReadLastMintDistribution(group string) (*common.MintDistribution, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	it := txn.NewIterator(opts)
	defer it.Close()

	dist := &common.MintDistribution{Group: group}
	it.Seek(graphMintKey(group, ^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixMint + group)) {
		item := it.Item()
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		var data common.MintDistribution
		err = msgpack.Unmarshal(ival, &data)
		if err != nil {
			return nil, err
		}
		dist.Batch = graphMintBatch(item.Key(), group)
		dist.Transaction = data.Transaction
		dist.Amount = data.Amount
	}
	return dist, nil
}

func (s *BadgerStore) LockMintInput(mint *common.MintData, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		dist, err := readMintInput(txn, mint)
		if err == badger.ErrKeyNotFound {
			return writeMintDistribution(txn, mint, tx)
		}
		if err != nil {
			return err
		}

		if dist.Transaction == tx && dist.Amount.Cmp(mint.Amount) == 0 {
			return nil
		}

		if !fork {
			return fmt.Errorf("mint locked for transaction %s amount %s", dist.Transaction.String(), dist.Amount.String())
		}
		err = pruneTransaction(txn, dist.Transaction)
		if err != nil {
			return err
		}
		return writeMintDistribution(txn, mint, tx)
	})
}

func readMintInput(txn *badger.Txn, mint *common.MintData) (*common.MintDistribution, error) {
	key := graphMintKey(mint.Group, mint.Batch)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	var dist common.MintDistribution
	err = msgpack.Unmarshal(ival, &dist)
	return &dist, err
}

func writeMintDistribution(txn *badger.Txn, mint *common.MintData, tx crypto.Hash) error {
	key := graphMintKey(mint.Group, mint.Batch)
	val := common.MsgpackMarshalPanic(mint.Distribute(tx))
	return txn.Set(key, val)
}

func graphMintKey(group string, batch uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, batch)
	key := append([]byte(group), buf...)
	return append([]byte(graphPrefixMint), key...)
}

func graphMintBatch(key []byte, group string) uint64 {
	batch := key[len(graphPrefixMint)+len(group):]
	return binary.BigEndian.Uint64(batch)
}
