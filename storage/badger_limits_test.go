package storage

import (
	"fmt"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestCacheTransactionSizeErrors(t *testing.T) {
	for _, target := range []int64{1, 2, 3} {
		t.Run(fmt.Sprintf("write %d", target), func(t *testing.T) {
			store := newTestBadgerStore(t)
			require.NoError(t, store.cacheDB.Close())
			store.cacheDB = openBadgerWithBatchCount(t, target)

			tx := common.NewTransactionV5(common.XINAssetId)
			tx.Inputs = []*common.Input{{Genesis: []byte("limited cache")}}
			tx.Extra = []byte{byte(target)}
			err := store.CacheQueueTransaction(tx.AsVersioned())
			require.ErrorIs(t, err, badger.ErrTxnTooBig)
		})
	}

	t.Run("remove", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.cacheDB.Close())
		store.cacheDB = openBadgerWithBatchCount(t, 3)
		hashes := []crypto.Hash{
			crypto.Blake3Hash([]byte("limited remove one")),
			crypto.Blake3Hash([]byte("limited remove two")),
		}
		err := store.CacheRemoveTransactions(hashes)
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})

	t.Run("remove first key", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.cacheDB.Close())
		store.cacheDB = openBadgerWithBatchCount(t, 1)
		err := store.CacheRemoveTransactions([]crypto.Hash{crypto.Blake3Hash([]byte("limited first remove"))})
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})

	t.Run("retrieve", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.cacheDB.Close())
		store.cacheDB = openBadgerWithBatchCount(t, 4)
		batch := store.cacheDB.NewWriteBatch()
		for i := range 3 {
			hash := crypto.Blake3Hash([]byte{byte(i + 1)})
			require.NoError(t, batch.Set(cacheTransactionQueueKey(uint64(i+1), hash), nil))
		}
		require.NoError(t, batch.Flush())

		_, err := store.CacheRetrieveTransactions(10)
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})

	t.Run("queued malformed transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		hash := crypto.Blake3Hash([]byte("queued malformed transaction"))
		require.NoError(t, store.cacheDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(cacheTransactionQueueKey(1, hash), nil); err != nil {
				return err
			}
			return txn.Set(cacheTransactionCacheKey(hash), []byte{0})
		}))
		_, err := store.CacheRetrieveTransactions(1)
		require.Error(t, err)
	})
}

func TestStorageTransactionSizeErrors(t *testing.T) {
	t.Run("topology", func(t *testing.T) {
		db := openBadgerWithBatchCount(t, 4)
		txn := fullBadgerTransaction(t, db)
		defer txn.Discard()
		tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
		snap := snapshotWithTopoForTx(crypto.Blake3Hash([]byte("limited topology")), 0, 1, 1, tx)
		require.ErrorIs(t, writeTopology(txn, snap), badger.ErrTxnTooBig)
	})

	t.Run("finalization", func(t *testing.T) {
		db := openBadgerWithBatchCount(t, 4)
		txn := fullBadgerTransaction(t, db)
		defer txn.Discard()
		tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
		snap := snapshotWithTopoForTx(crypto.Blake3Hash([]byte("limited finalization")), 0, 1, 1, tx)
		require.ErrorIs(t, finalizeTransaction(txn, tx, snap), badger.ErrTxnTooBig)
	})

	t.Run("new round link", func(t *testing.T) {
		db := openBadgerWithBatchCount(t, 4)
		node := crypto.Blake3Hash([]byte("limited round node"))
		externalHash := crypto.Blake3Hash([]byte("limited round external"))
		externalNode := crypto.Blake3Hash([]byte("limited round external node"))
		seedBadger(t, db,
			func(txn *badger.Txn) error {
				return writeRound(txn, node, &common.Round{Hash: node, NodeId: node, References: &common.RoundLink{}})
			},
			func(txn *badger.Txn) error {
				return writeRound(txn, externalHash, &common.Round{Hash: externalHash, NodeId: externalNode, References: &common.RoundLink{}})
			},
		)
		txn := fullBadgerTransaction(t, db)
		defer txn.Discard()
		refs := &common.RoundLink{Self: crypto.Blake3Hash([]byte("limited round self")), External: externalHash}
		require.ErrorIs(t, startNewRound(txn, node, 1, refs, 1), badger.ErrTxnTooBig)
	})

	t.Run("snapshot unique index", func(t *testing.T) {
		db := openBadgerWithBatchCount(t, 4)
		node := crypto.Blake3Hash([]byte("limited snapshot node"))
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: []byte("limited snapshot")}}
		ver := tx.AsVersioned()
		snap := snapshotWithTopoForTx(node, 0, 1, 1, ver)
		seedBadger(t, db, func(txn *badger.Txn) error {
			if err := txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()); err != nil {
				return err
			}
			hash := snap.PayloadHash()
			return txn.Set(graphFinalizationKey(ver.PayloadHash()), hash[:])
		})
		txn := fullBadgerTransaction(t, db)
		defer txn.Discard()
		require.ErrorIs(t, writeSnapshot(txn, snap), badger.ErrTxnTooBig)
	})

	for _, target := range []int64{3, 4} {
		t.Run(fmt.Sprintf("snapshot write stage %d", target), func(t *testing.T) {
			db := openBadgerWithBatchCount(t, target)
			node := crypto.Blake3Hash([]byte{byte(target), 1})
			tx := common.NewTransactionV5(common.XINAssetId)
			tx.Inputs = []*common.Input{{Genesis: []byte("limited snapshot stage")}}
			ver := tx.AsVersioned()
			snap := snapshotWithTopoForTx(node, 0, 1, 1, ver)
			batch := db.NewWriteBatch()
			require.NoError(t, batch.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()))
			hash := snap.PayloadHash()
			require.NoError(t, batch.Set(graphFinalizationKey(ver.PayloadHash()), hash[:]))
			require.NoError(t, batch.Flush())
			err := db.Update(func(txn *badger.Txn) error { return writeSnapshot(txn, snap) })
			require.ErrorIs(t, err, badger.ErrTxnTooBig)
		})
	}

	t.Run("snapshot work removal", func(t *testing.T) {
		db := openBadgerWithBatchCount(t, 4)
		node := crypto.Blake3Hash([]byte("limited work node"))
		seedBadger(t, db, func(txn *badger.Txn) error {
			return txn.Set(graphWorkSnapshotKey(node, 1, 1), []byte{1})
		})
		txn := fullBadgerTransaction(t, db)
		defer txn.Discard()
		require.ErrorIs(t, removeSnapshotWorksForRound(txn, node, 1), badger.ErrTxnTooBig)
	})

	t.Run("public writers", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 1)
		node := crypto.Blake3Hash([]byte("limited public writer"))
		require.ErrorIs(t, store.StartNewRound(node, 0, &common.RoundLink{}, 0), badger.ErrTxnTooBig)

		tx := nodeOpTransaction(common.OutputTypeNodePledge)
		require.ErrorIs(t, store.AddNodeOperation(tx, 2, 1, false), badger.ErrTxnTooBig)
	})

	t.Run("empty round update", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 3)
		node := crypto.Blake3Hash([]byte("limited empty round"))
		externalHash := crypto.Blake3Hash([]byte("limited empty external"))
		externalNode := crypto.Blake3Hash([]byte("limited empty external node"))
		refs := &common.RoundLink{
			Self:     crypto.Blake3Hash([]byte("limited empty self")),
			External: externalHash,
		}
		seedBadger(t, store.snapshotsDB,
			func(txn *badger.Txn) error {
				return writeRound(txn, node, &common.Round{Hash: node, NodeId: node, Number: 1, References: refs})
			},
			func(txn *badger.Txn) error {
				return writeRound(txn, externalHash, &common.Round{Hash: externalHash, NodeId: externalNode, References: &common.RoundLink{}})
			},
		)
		require.ErrorIs(t, store.UpdateEmptyHeadRound(node, 1, refs), badger.ErrTxnTooBig)
	})

	t.Run("graph removal", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 3)
		batch := store.snapshotsDB.NewWriteBatch()
		require.NoError(t, batch.Set([]byte("LIMITED-GRAPH-ENTRY-1"), nil))
		require.NoError(t, batch.Set([]byte("LIMITED-GRAPH-ENTRY-2"), nil))
		require.NoError(t, batch.Set([]byte("LIMITED-GRAPH-ENTRY-3"), nil))
		require.NoError(t, batch.Flush())
		_, err := store.RemoveGraphEntries("LIMITED-GRAPH")
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})

	t.Run("round work removal", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 3)
		node := crypto.Blake3Hash([]byte("limited work removal"))
		batch := store.snapshotsDB.NewWriteBatch()
		for i := range 4 {
			require.NoError(t, batch.Set(graphWorkSnapshotKey(node, 0, uint64(i+1)), []byte{1}))
		}
		require.NoError(t, batch.Flush())
		err := store.WriteRoundWork(node, 1, nil, false)
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})

	t.Run("genesis round", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 3)
		node := crypto.Blake3Hash([]byte("limited genesis round"))
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: []byte("limited genesis")}}
		tx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
		ver := tx.AsVersioned()
		snap := snapshotWithTopoForTx(node, 0, 0, 1, ver)
		round := &common.Round{Hash: node, NodeId: node, References: &common.RoundLink{}}
		err := store.LoadGenesis([]*common.Round{round}, []*common.SnapshotWithTopologicalOrder{snap}, []*common.VersionedTransaction{ver})
		require.ErrorIs(t, err, badger.ErrTxnTooBig)
	})
}

func openBadgerWithBatchCount(t *testing.T, target int64) *badger.DB {
	t.Helper()
	for size := int64(512); size <= 64<<10; size += 64 {
		opts := badger.DefaultOptions(t.TempDir()).
			WithLogger(nil).
			WithMemTableSize(size).
			WithValueThreshold(1)
		db, err := badger.Open(opts)
		require.NoError(t, err)
		if db.MaxBatchCount() == target {
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			return db
		}
		require.NoError(t, db.Close())
	}
	t.Fatalf("no Badger memtable size produced batch count %d", target)
	return nil
}

func fullBadgerTransaction(t *testing.T, db *badger.DB) *badger.Txn {
	t.Helper()
	txn := db.NewTransaction(true)
	for i := byte(1); ; i++ {
		err := txn.Set([]byte{i}, nil)
		if err == badger.ErrTxnTooBig {
			return txn
		}
		require.NoError(t, err)
	}
}

func seedBadger(t *testing.T, db *badger.DB, writes ...func(*badger.Txn) error) {
	t.Helper()
	for _, write := range writes {
		require.NoError(t, db.Update(write))
	}
}
