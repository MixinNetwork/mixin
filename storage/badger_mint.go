package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
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
	if it.ValidForPrefix([]byte(graphPrefixMint)) {
		item := it.Item()
		dist.Batch = graphMintBatch(item.Key(), group)
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		copy(dist.Transaction[:], ival)
	}
	return dist, nil
}

func (s *BadgerStore) LockMintInput(mint *common.MintData, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphMintKey(mint.Group, mint.Batch)
		ival, err := readMintInput(txn, mint)

		if err == badger.ErrKeyNotFound {
			return txn.Set(key, tx[:])
		}
		if err != nil {
			return err
		}

		if bytes.Compare(ival, tx[:]) != 0 {
			if !fork {
				return fmt.Errorf("mint locked for transaction %s", hex.EncodeToString(ival))
			}
			var hash crypto.Hash
			copy(hash[:], ival)
			err := pruneTransaction(txn, hash)
			if err != nil {
				return err
			}
		}
		return txn.Set(key, tx[:])
	})
}

func readMintInput(txn *badger.Txn, mint *common.MintData) ([]byte, error) {
	key := graphMintKey(mint.Group, mint.Batch)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
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
