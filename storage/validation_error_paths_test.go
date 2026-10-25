package storage

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestValidateGraphEntriesCollectsCountsAndErrors(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			store := newTestBadgerStore(t)
			network := crypto.Blake3Hash([]byte("validation network"))
			require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
				for i := range count {
					key := seededPublicKey(byte(23 + i))
					if err := writeNodeAccept(txn, key, key, crypto.Blake3Hash([]byte{byte(i)}), uint64(i+1), true); err != nil {
						return err
					}
				}
				return nil
			}))
			nodes := store.ReadAllNodes(^uint64(0), false)
			require.Len(t, nodes, count)
			require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
				for _, n := range nodes {
					id := n.IdForNetwork(network)
					tx := common.NewTransactionV5(common.XINAssetId).AsVersioned()
					snap := snapshotWithTopoForTx(id, 0, 1, 1, tx)
					if err := writeRound(txn, id, &common.Round{Hash: id, NodeId: id, Number: 1, References: &common.RoundLink{}}); err != nil {
						return err
					}
					// The snapshot's transaction is deliberately absent.
					if err := txn.Set(graphSnapshotKey(id, 0, snap.PayloadHash()), snap.VersionedMarshal()); err != nil {
						return err
					}
				}
				return nil
			}))
			total, invalid, err := store.ValidateGraphEntries(network, 1)
			if count == 0 {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, badger.ErrKeyNotFound)
			}
			require.Equal(t, count, total)
			require.Zero(t, invalid)
		})
	}
}

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

	t.Run("missing round snapshots", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(writeHead))
		total, invalid, err := store.validateSnapshotEntriesForNode(node, 1)
		require.ErrorContains(t, err, "missing snapshots for node")
		require.Zero(t, total)
		require.Zero(t, invalid)
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

func TestValidateGraphEntriesCorruptReferences(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		total   int
	}{
		{"missing finalization snapshot", "missing finalization snapshot", 1},
		{"malformed finalization hash", "malformed finalization", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestBadgerStore(t)
			network := crypto.Blake3Hash([]byte("corrupt graph network"))
			signer := seededNodeAddress(221)
			node := signer.Hash().ForNetwork(network)
			require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
				if err := writeNodeAccept(txn, signer.PublicSpendKey, signer.PublicSpendKey, node, 1, true); err != nil {
					return err
				}
				if err := writeRound(txn, node, &common.Round{Hash: node, NodeId: node, Number: 1, References: &common.RoundLink{}}); err != nil {
					return err
				}
				for i := range tc.total {
					ver := common.NewTransactionV5(common.XINAssetId).AsVersioned()
					ver.Extra = []byte{byte(i)}
					snap := snapshotWithTopoForTx(node, 0, uint64(i+1), 1+uint64(i)*config.SnapshotRoundGap, ver)
					key := graphSnapshotKey(node, 0, snap.PayloadHash())
					if err := txn.Set(key, snap.VersionedMarshal()); err != nil {
						return err
					}
					if err := txn.Set(graphTransactionKey(ver.PayloadHash()), ver.Marshal()); err != nil {
						return err
					}
					hash := snap.PayloadHash()
					finalization := hash[:]
					if tc.name == "malformed finalization hash" {
						finalization = []byte{1}
					}
					if err := txn.Set(graphFinalizationKey(ver.PayloadHash()), finalization); err != nil {
						return err
					}
				}
				return nil
			}))
			total, invalid, err := store.ValidateGraphEntries(network, 1)
			require.ErrorContains(t, err, tc.message)
			require.Equal(t, tc.total, total)
			require.Zero(t, invalid)
		})
	}
}
