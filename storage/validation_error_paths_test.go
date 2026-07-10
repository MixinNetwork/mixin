package storage

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestSnapshotValidationErrorPaths(t *testing.T) {
	node := crypto.Blake3Hash([]byte("validation error node"))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte("validation error")}}
	tx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	ver := tx.AsVersioned()
	snap := snapshotWithTopoForTx(node, 0, 1, 1, ver)

	writeHead := func(txn *badger.Txn) error {
		return writeRound(txn, node, &common.Round{
			Hash:       node,
			NodeId:     node,
			Number:     1,
			References: &common.RoundLink{},
		})
	}
	writeSnapshotRecord := func(txn *badger.Txn) error {
		key := graphSnapshotKey(node, 0, snap.PayloadHash())
		return txn.Set(key, snap.VersionedMarshal())
	}
	writeSnapshotTopology := func(txn *badger.Txn) error {
		key := graphSnapshotKey(node, 0, snap.PayloadHash())
		topology := graphTopologyKey(1)
		if err := txn.Set(key, snap.VersionedMarshal()); err != nil {
			return err
		}
		if err := txn.Set(topology, key); err != nil {
			return err
		}
		return txn.Set(graphSnapTopologyKey(snap.PayloadHash()), topology)
	}

	t.Run("head round", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphRoundKey(node), []byte{0})
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})

	t.Run("snapshot", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			return txn.Set(graphSnapshotKey(node, 0, snap.PayloadHash()), []byte{0})
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})

	t.Run("missing transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			return writeSnapshotRecord(txn)
		}))
		total, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Equal(t, 1, total)
		require.Error(t, err)
	})

	t.Run("malformed transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			if err := writeSnapshotRecord(txn); err != nil {
				return err
			}
			return txn.Set(graphTransactionKey(ver.PayloadHash()), []byte{0})
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})

	t.Run("missing finalization", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			if err := writeSnapshotRecord(txn); err != nil {
				return err
			}
			return txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal())
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})

	t.Run("missing finalization topology", func(t *testing.T) {
		store := newTestBadgerStore(t)
		duplicate := crypto.Blake3Hash([]byte("missing finalization topology"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			if err := writeSnapshotRecord(txn); err != nil {
				return err
			}
			if err := txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()); err != nil {
				return err
			}
			if err := txn.Set(graphFinalizationKey(ver.PayloadHash()), duplicate[:]); err != nil {
				return err
			}
			return txn.Set(graphSnapTopologyKey(duplicate), graphTopologyKey(99))
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})

	t.Run("missing round", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			if err := writeSnapshotTopology(txn); err != nil {
				return err
			}
			if err := txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()); err != nil {
				return err
			}
			hash := snap.PayloadHash()
			return txn.Set(graphFinalizationKey(ver.PayloadHash()), hash[:])
		}))
		total, invalid, err := store.validateSnapshotEntriesForNode(node, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, 1, invalid)
	})

	t.Run("malformed computed round", func(t *testing.T) {
		store := newTestBadgerStore(t)
		_, _, roundHash := computeRoundHash(node, 0, []*common.SnapshotWithTopologicalOrder{snap})
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeHead(txn); err != nil {
				return err
			}
			if err := writeSnapshotTopology(txn); err != nil {
				return err
			}
			if err := txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()); err != nil {
				return err
			}
			hash := snap.PayloadHash()
			if err := txn.Set(graphFinalizationKey(ver.PayloadHash()), hash[:]); err != nil {
				return err
			}
			return txn.Set(graphRoundKey(roundHash), []byte{0})
		}))
		_, _, err := store.validateSnapshotEntriesForNode(node, 1)
		require.Error(t, err)
	})
}

func TestComputeRoundHashOrderingAndVersionSelection(t *testing.T) {
	node := crypto.Blake3Hash([]byte("round hash ordering"))
	lateHash := crypto.Blake3Hash([]byte("z hash"))
	earlyHash := crypto.Blake3Hash([]byte("a hash"))
	snapshots := []*common.SnapshotWithTopologicalOrder{
		{Snapshot: &common.Snapshot{Version: common.SnapshotVersionCommonEncoding, Timestamp: 1, Hash: lateHash}},
		{Snapshot: &common.Snapshot{Version: common.SnapshotVersionCommonEncoding + 1, Timestamp: 1, Hash: earlyHash}},
	}
	start, end, hash := computeRoundHash(node, 1, snapshots)
	require.Equal(t, uint64(1), start)
	require.Equal(t, uint64(1), end)
	require.True(t, hash.HasValue())
	require.LessOrEqual(t, bytes.Compare(snapshots[0].Hash[:], snapshots[1].Hash[:]), 0)
}
