package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v4"
)

// FIXME remove this in the future
func (s *BadgerStore) OneTimeFixMintPrefix() error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	mints, err := s.readOldMintDistributions(txn)
	if err != nil || len(mints) == 0 {
		return err
	}

	for _, m := range mints {
		err = writeMintDistribution(txn, &m.MintData, m.Transaction)
		if err != nil {
			return err
		}
		key := binary.BigEndian.AppendUint64([]byte("MINTKERNELNODE"), m.Batch)
		err = txn.Delete(key)
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *BadgerStore) readOldMintDistributions(txn *badger.Txn) ([]*common.MintDistribution, error) {
	mints := make([]*common.MintDistribution, 0)

	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte("MINTKERNELNODE")
	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(opts.Prefix)
	for ; it.ValidForPrefix(opts.Prefix); it.Next() {
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
		if data.Batch != binary.BigEndian.Uint64(key[len(opts.Prefix):]) {
			panic("malformed mint data")
		}

		mints = append(mints, data)
	}

	return mints, nil
}
