package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestCacheQueueDeduplicationAndCorruption(t *testing.T) {
	store := newTestBadgerStore(t)
	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	hash := tx.PayloadHash()
	require.NoError(t, store.CachePutTransaction(tx))

	orphan := crypto.Blake3Hash([]byte("orphan cache queue entry"))
	err := store.cacheDB.Update(func(txn *badger.Txn) error {
		if err := txn.Set(cacheTransactionQueueKey(1, hash), nil); err != nil {
			return err
		}
		if err := txn.Set(cacheTransactionQueueKey(2, orphan), nil); err != nil {
			return err
		}
		return txn.Set(cacheTransactionOrderKey(orphan), nil)
	})
	require.NoError(t, err)

	retrieved, err := store.CacheRetrieveTransactions(10)
	require.NoError(t, err)
	require.Len(t, retrieved, 1)
	require.Equal(t, hash, retrieved[0].PayloadHash())

	retrieved, err = store.CacheRetrieveTransactions(10)
	require.NoError(t, err)
	require.Empty(t, retrieved)

	corrupt := crypto.Blake3Hash([]byte("corrupt cache transaction"))
	err = store.cacheDB.Update(func(txn *badger.Txn) error {
		return txn.Set(cacheTransactionCacheKey(corrupt), []byte{0xff})
	})
	require.NoError(t, err)
	got, err := store.CacheGetTransaction(corrupt)
	require.Error(t, err)
	require.Nil(t, got)
}

func TestCacheRemoveTransactionsInBatches(t *testing.T) {
	store := newTestBadgerStore(t)
	hashes := make([]crypto.Hash, 101)
	for i := range hashes {
		hashes[i] = crypto.Blake3Hash([]byte{byte(i)})
	}

	err := store.cacheDB.Update(func(txn *badger.Txn) error {
		for _, index := range []int{0, 100} {
			if err := txn.Set(cacheTransactionCacheKey(hashes[index]), []byte("payload")); err != nil {
				return err
			}
			if err := txn.Set(cacheTransactionOrderKey(hashes[index]), nil); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, store.CacheRemoveTransactions(hashes))

	err = store.cacheDB.View(func(txn *badger.Txn) error {
		for _, index := range []int{0, 100} {
			_, err := txn.Get(cacheTransactionCacheKey(hashes[index]))
			require.ErrorIs(t, err, badger.ErrKeyNotFound)
			_, err = txn.Get(cacheTransactionOrderKey(hashes[index]))
			require.ErrorIs(t, err, badger.ErrKeyNotFound)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestCachePutTransactionAfterClose(t *testing.T) {
	store := newTestBadgerStore(t)
	require.NoError(t, store.cacheDB.Close())

	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	err := store.CachePutTransaction(tx)
	require.Error(t, err)
}

func TestStorageCorruptionAndKeyBounds(t *testing.T) {
	store := newTestBadgerStore(t)
	deposit := &common.DepositData{
		Chain:       common.BitcoinAssetId,
		AssetKey:    "btc",
		Transaction: "corrupt-deposit-lock",
		Index:       1,
	}
	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphDepositKey(deposit), []byte{1})
	})
	require.NoError(t, err)
	require.Panics(t, func() {
		_, _ = store.ReadDepositLock(deposit)
	})

	require.Panics(t, func() {
		graphUtxoKey(crypto.Hash{}, common.InputIndexLimit+1)
	})
}
