package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadMintDistributions(offset, count uint64) ([]*common.MintDistribution, []*common.VersionedTransaction, error) {
	if count > 500 {
		return nil, nil, fmt.Errorf("count %d too large, the maximum is 500", count)
	}

	mints := make([]*common.MintDistribution, 0)
	transactions := make([]*common.VersionedTransaction, 0)

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(graphPrefixMint)
	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphMintKey(offset))
	for ; it.Valid() && uint64(len(mints)) < count; it.Next() {
		item := it.Item()
		key := item.KeyCopy(nil)
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return nil, nil, err
		}
		data, err := common.DecompressUnmarshalMintDistribution(ival)
		if err != nil {
			return nil, nil, err
		}
		if data.Batch != graphMintBatch(key) {
			panic("malformed mint data")
		}

		tx, err := readTransaction(txn, data.Transaction)
		if err != nil {
			return nil, nil, err
		}
		if tx == nil {
			continue
		}
		_, err = txn.Get(graphFinalizationKey(data.Transaction))
		if err == badger.ErrKeyNotFound {
			continue
		} else if err != nil {
			return nil, nil, err
		}

		transactions = append(transactions, tx)
		mints = append(mints, data)
	}

	return mints, transactions, nil
}

func (s *BadgerStore) ReadLastMintDistribution() (*common.MintDistribution, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Reverse = true
	opts.Prefix = []byte(graphPrefixMint)
	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphMintKey(^uint64(0)))
	for ; it.Valid(); it.Next() {
		item := it.Item()
		key := item.KeyCopy(nil)
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return nil, err
		}
		data, err := common.DecompressUnmarshalMintDistribution(ival)
		if err != nil {
			return nil, err
		}
		if data.Batch != graphMintBatch(key) {
			panic("malformed mint data")
		}
		_, err = txn.Get(graphFinalizationKey(data.Transaction))
		if err == badger.ErrKeyNotFound {
			continue
		} else if err != nil {
			return nil, err
		}
		return data, nil
	}

	return &common.MintDistribution{}, nil
}

func (s *BadgerStore) LockMintInput(mint *common.MintData, tx crypto.Hash, fork bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	key := graphMintKey(mint.Batch)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	return common.DecompressUnmarshalMintDistribution(ival)
}

func writeMintDistribution(txn *badger.Txn, mint *common.MintData, tx crypto.Hash) error {
	key := graphMintKey(mint.Batch)
	val := mint.Distribute(tx).CompressMarshal()
	return txn.Set(key, val)
}

func graphMintKey(batch uint64) []byte {
	key := []byte(graphPrefixMint)
	return binary.BigEndian.AppendUint64(key, batch)
}

func graphMintBatch(key []byte) uint64 {
	batch := key[len(graphPrefixMint):]
	return binary.BigEndian.Uint64(batch)
}
