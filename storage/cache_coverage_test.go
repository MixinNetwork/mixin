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
	require.NoError(t, store.CacheQueueTransaction(tx))

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

func TestCacheStoreTransactionRemainsUnqueuedUntilExplicitlyQueued(t *testing.T) {
	store := newTestBadgerStore(t)
	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	firstSignature := crypto.Signature{1}
	tx.SignaturesMap = []map[uint16]*crypto.Signature{{0: &firstSignature}}

	replacement := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	replacementSignature := crypto.Signature{2}
	replacement.SignaturesMap = []map[uint16]*crypto.Signature{{0: &replacementSignature}}
	require.Equal(t, tx.PayloadHash(), replacement.PayloadHash())
	require.NotEqual(t, tx.Marshal(), replacement.Marshal())

	require.NoError(t, store.CacheStoreTransaction(tx))
	cached, err := store.CacheGetTransaction(tx.PayloadHash())
	require.NoError(t, err)
	require.Equal(t, tx.Marshal(), cached.Marshal())

	retrieved, err := store.CacheRetrieveTransactions(1)
	require.NoError(t, err)
	require.Empty(t, retrieved)

	// Cache-only delivery is first-write-wins for an existing payload hash.
	require.NoError(t, store.CacheStoreTransaction(replacement))
	cached, err = store.CacheGetTransaction(tx.PayloadHash())
	require.NoError(t, err)
	require.Equal(t, tx.Marshal(), cached.Marshal())

	// Explicit queueing adds scheduling records but never replaces the
	// existing envelope, so a same-payload envelope with different
	// authorization cannot displace the first cached copy.
	require.NoError(t, store.CacheQueueTransaction(replacement))
	retrieved, err = store.CacheRetrieveTransactions(1)
	require.NoError(t, err)
	require.Len(t, retrieved, 1)
	require.Equal(t, tx.Marshal(), retrieved[0].Marshal())
	require.NotEqual(t, replacement.Marshal(), retrieved[0].Marshal())

	retrieved, err = store.CacheRetrieveTransactions(1)
	require.NoError(t, err)
	require.Empty(t, retrieved)
}

func TestCacheRemoveTransactionsInBatches(t *testing.T) {
	store := newTestBadgerStore(t)
	require.NoError(t, store.cacheDB.Close())
	store.cacheDB = openBadgerWithBatchCount(t, 202)
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

func TestCacheRetrievalProgressesPastStaleEntries(t *testing.T) {
	store := newTestBadgerStore(t)
	require.NoError(t, store.cacheDB.Close())
	store.cacheDB = openBadgerWithBatchCount(t, 6)

	// Finalization removes cached payloads but leaves queue records behind.
	// Each retrieval must bound the deletions even when no live payload is found.
	batch := store.cacheDB.NewWriteBatch()
	defer batch.Cancel()
	for i := range 10 {
		hash := crypto.Blake3Hash([]byte{byte(i)})
		require.NoError(t, batch.Set(cacheTransactionQueueKey(uint64(i+1), hash), nil))
	}
	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	hash := tx.PayloadHash()
	require.NoError(t, batch.Set(cacheTransactionQueueKey(11, hash), nil))
	require.NoError(t, batch.Set(cacheTransactionCacheKey(hash), tx.Marshal()))
	require.NoError(t, batch.Flush())

	for range 5 {
		txs, err := store.CacheRetrieveTransactions(2)
		require.NoError(t, err)
		require.Empty(t, txs)
	}
	txs, err := store.CacheRetrieveTransactions(2)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, hash, txs[0].PayloadHash())
	txs, err = store.CacheRetrieveTransactions(2)
	require.NoError(t, err)
	require.Empty(t, txs)
}

func TestCacheQueueTransactionAfterClose(t *testing.T) {
	store := newTestBadgerStore(t)
	require.NoError(t, store.cacheDB.Close())

	tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	err := store.CacheQueueTransaction(tx)
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
