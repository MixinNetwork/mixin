package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestWriteTransactionRejectsMissingOrMalformedLocks(t *testing.T) {
	output := func() []*common.Output {
		return []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	}

	t.Run("deposit", func(t *testing.T) {
		store := newTestBadgerStore(t)
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Deposit: &common.DepositData{
			Chain:       common.XINAsset.Chain,
			AssetKey:    common.XINAsset.AssetKey,
			Transaction: "missing deposit lock",
			Amount:      common.NewInteger(1),
		}}}
		tx.Outputs = output()
		require.Panics(t, func() { _ = store.WriteTransaction(tx.AsVersioned()) })
	})

	t.Run("mint", func(t *testing.T) {
		store := newTestBadgerStore(t)
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.AddUniversalMintInput(99, common.NewInteger(1))
		tx.Outputs = output()
		require.Panics(t, func() { _ = store.WriteTransaction(tx.AsVersioned()) })
	})

	t.Run("missing utxo", func(t *testing.T) {
		store := newTestBadgerStore(t)
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.AddInput(crypto.Blake3Hash([]byte("missing utxo")), 0)
		tx.Outputs = output()
		require.Panics(t, func() { _ = store.WriteTransaction(tx.AsVersioned()) })
	})

	t.Run("malformed utxo", func(t *testing.T) {
		store := newTestBadgerStore(t)
		hash := crypto.Blake3Hash([]byte("malformed utxo"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphUtxoKey(hash, 0), []byte{0})
		}))
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.AddInput(hash, 0)
		tx.Outputs = output()
		require.Panics(t, func() { _ = store.WriteTransaction(tx.AsVersioned()) })
	})
}

func TestTransactionFinalizationAndOutputBranches(t *testing.T) {
	t.Run("duplicate write and finalization", func(t *testing.T) {
		store := newTestBadgerStore(t)
		node := crypto.Blake3Hash([]byte("duplicate finalization node"))
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: []byte("duplicate finalization")}}
		tx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
		ver := tx.AsVersioned()
		snap := snapshotWithTopoForTx(node, 0, 1, 1, ver)

		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
				return err
			}
			require.NoError(t, writeTransaction(txn, ver))
			require.NoError(t, writeTransaction(txn, ver))
			require.NoError(t, finalizeTransaction(txn, ver, snap))
			require.NoError(t, finalizeTransaction(txn, ver, snap))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("deposit asset conflict", func(t *testing.T) {
		store := newTestBadgerStore(t)
		assetID := crypto.Blake3Hash([]byte("deposit asset conflict"))
		deposit := &common.DepositData{
			Chain:       common.BitcoinAssetId,
			AssetKey:    "expected",
			Transaction: "deposit asset conflict",
			Amount:      common.NewInteger(1),
		}
		tx := common.NewTransactionV5(assetID)
		tx.Inputs = []*common.Input{{Deposit: deposit}}
		tx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
		ver := tx.AsVersioned()
		snap := snapshotWithTopoForTx(crypto.Blake3Hash([]byte("deposit node")), 0, 1, 1, ver)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeAssetInfo(txn, assetID, &common.Asset{Chain: common.BitcoinAssetId, AssetKey: "different"}); err != nil {
				return err
			}
			return finalizeTransaction(txn, ver, snap)
		})
		require.ErrorContains(t, err, "invalid asset info")
	})

	t.Run("node output dispatch", func(t *testing.T) {
		store := newTestBadgerStore(t)
		signer, payee := seededAddress(220), seededAddress(221)
		extra := append(append([]byte(nil), signer.PublicSpendKey[:]...), payee.PublicSpendKey[:]...)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			for i, typ := range []uint8{
				common.OutputTypeNodePledge,
				common.OutputTypeNodeCancel,
				common.OutputTypeNodeAccept,
				common.OutputTypeNodeRemove,
			} {
				hash := crypto.Blake3Hash([]byte{byte(i + 1), typ})
				tx := common.NewTransactionV5(common.XINAssetId)
				tx.Inputs = []*common.Input{{Genesis: []byte("node output")}}
				tx.Outputs = []*common.Output{{Type: typ, Amount: common.NewInteger(1)}}
				tx.Extra = extra
				ver := tx.AsVersioned()
				utxo := &common.UTXOWithLock{UTXO: common.UTXO{
					Input:  common.Input{Hash: hash},
					Output: common.Output{Type: typ, Amount: common.NewInteger(1)},
				}}
				if err := writeUTXO(txn, utxo, ver, uint64(i+1), typ == common.OutputTypeNodeAccept); err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("ghost output failure and compatibility exception", func(t *testing.T) {
		store := newTestBadgerStore(t)
		ghost := seededPublicKey(222)
		lockedBy := crypto.Blake3Hash([]byte("existing ghost owner"))
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphGhostKey(ghost), lockedBy[:]); err != nil {
				return err
			}
			tx := common.NewTransactionV5(common.XINAssetId)
			tx.Inputs = []*common.Input{{Genesis: []byte("ghost output")}}
			ver := tx.AsVersioned()
			utxo := &common.UTXOWithLock{UTXO: common.UTXO{
				Input: common.Input{Hash: crypto.Blake3Hash([]byte("new ghost owner"))},
				Output: common.Output{
					Type:   common.OutputTypeScript,
					Amount: common.NewInteger(1),
					Keys:   []*crypto.Key{&ghost},
				},
			}}
			require.Error(t, writeUTXO(txn, utxo, ver, 1, false))

			compat, err := crypto.HashFromString("c63b6373652def5999c1d951fcb8f064db67b7d18565847b921b21639e15dddd")
			require.NoError(t, err)
			require.NoError(t, lockGhostKey(txn, &ghost, compat, true))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("finalized deposit lock cannot be replaced", func(t *testing.T) {
		store := newTestBadgerStore(t)
		deposit := &common.DepositData{
			Chain:       common.BitcoinAssetId,
			AssetKey:    "btc",
			Transaction: "finalized deposit",
			Amount:      common.NewInteger(1),
		}
		old := crypto.Blake3Hash([]byte("finalized deposit transaction"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeDepositLock(txn, deposit, old); err != nil {
				return err
			}
			return txn.Set(graphFinalizationKey(old), old[:])
		}))
		err := store.LockDepositInput(deposit, crypto.Blake3Hash([]byte("replacement deposit")), true)
		require.ErrorContains(t, err, "prune finalized transaction")
	})
}

func TestConsensusSnapshotGuardsAndReadErrors(t *testing.T) {
	node := crypto.Blake3Hash([]byte("consensus guard node"))
	genesisTx := common.NewTransactionV5(common.XINAssetId)
	genesisTx.Inputs = []*common.Input{{Genesis: []byte("consensus genesis")}}
	genesisTx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
	genesis := genesisTx.AsVersioned()
	genesisSnap := snapshotWithTopoForTx(node, 0, 1, 1, genesis)

	nonGenesisTx := common.NewTransactionV5(common.XINAssetId)
	nonGenesisTx.AddInput(crypto.Blake3Hash([]byte("consensus input")), 0)
	nonGenesisTx.Outputs = []*common.Output{{Type: common.OutputTypeNodeRemove, Amount: common.NewInteger(1)}}
	nonGenesisTx.References = []crypto.Hash{crypto.Blake3Hash([]byte("consensus reference"))}
	nonGenesis := nonGenesisTx.AsVersioned()
	nonGenesisSnap := snapshotWithTopoForTx(node, 1, 2, 10, nonGenesis)

	t.Run("shape and transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
			require.Panics(t, func() {
				_ = writeConsensusSnapshot(txn, &common.Snapshot{
					Version: common.SnapshotVersionCommonEncoding,
					NodeId:  node,
				}, genesis, nil)
			})
			require.Panics(t, func() {
				_ = writeConsensusSnapshot(txn, &common.Snapshot{
					Version:      common.SnapshotVersionCommonEncoding,
					NodeId:       node,
					RoundNumber:  1,
					Transactions: []crypto.Hash{genesis.PayloadHash(), genesis.PayloadHash()},
				}, genesis, nil)
			})
			wrong := *genesisSnap.Snapshot
			wrong.Transactions = []crypto.Hash{crypto.Blake3Hash([]byte("wrong transaction"))}
			require.Panics(t, func() { _ = writeConsensusSnapshot(txn, &wrong, genesis, nil) })
			return nil
		}))
	})

	t.Run("chain linkage", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
			hackMany := &common.Snapshot{
				Version:      common.SnapshotVersionCommonEncoding,
				NodeId:       node,
				RoundNumber:  1,
				Timestamp:    1,
				Transactions: []crypto.Hash{nonGenesis.PayloadHash(), genesis.PayloadHash()},
			}
			require.Panics(t, func() { _ = writeConsensusSnapshot(txn, nonGenesisSnap.Snapshot, nonGenesis, hackMany) })

			hackSame := &common.Snapshot{
				Version:      common.SnapshotVersionCommonEncoding,
				NodeId:       node,
				Timestamp:    1,
				Transactions: []crypto.Hash{nonGenesis.PayloadHash()},
			}
			require.NoError(t, writeConsensusSnapshot(txn, nonGenesisSnap.Snapshot, nonGenesis, hackSame))

			hackWrong := &common.Snapshot{
				Version:      common.SnapshotVersionCommonEncoding,
				NodeId:       node,
				Timestamp:    1,
				Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("wrong predecessor"))},
			}
			require.Panics(t, func() { _ = writeConsensusSnapshot(txn, nonGenesisSnap.Snapshot, nonGenesis, hackWrong) })

			hackLate := &common.Snapshot{
				Version:      common.SnapshotVersionCommonEncoding,
				NodeId:       node,
				Timestamp:    nonGenesisSnap.Timestamp,
				Transactions: []crypto.Hash{nonGenesis.References[0]},
			}
			require.Panics(t, func() { _ = writeConsensusSnapshot(txn, nonGenesisSnap.Snapshot, nonGenesis, hackLate) })
			return nil
		}))
	})

	t.Run("hack cannot replace stored head", func(t *testing.T) {
		store := newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			snapshotKey := graphSnapshotKey(genesisSnap.NodeId, genesisSnap.RoundNumber, genesisSnap.PayloadHash())
			topologyKey := graphTopologyKey(genesisSnap.TopologicalOrder)
			if err := txn.Set(snapshotKey, genesisSnap.VersionedMarshal()); err != nil {
				return err
			}
			if err := txn.Set(topologyKey, snapshotKey); err != nil {
				return err
			}
			if err := txn.Set(graphSnapTopologyKey(genesisSnap.PayloadHash()), topologyKey); err != nil {
				return err
			}
			if err := txn.Set(graphConsensusSnapshotKey(genesisSnap.Timestamp, genesisSnap.PayloadHash()), nil); err != nil {
				return err
			}
			require.Panics(t, func() {
				_ = writeConsensusSnapshot(txn, genesisSnap.Snapshot, genesis, genesisSnap.Snapshot)
			})
			return nil
		})
		require.NoError(t, err)
	})

	corruptConsensusStore := func(t *testing.T) *BadgerStore {
		t.Helper()
		store := newTestBadgerStore(t)
		hash := crypto.Blake3Hash([]byte("missing consensus topology"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphSnapTopologyKey(hash), graphTopologyKey(99)); err != nil {
				return err
			}
			return txn.Set(graphConsensusSnapshotKey(1, hash), nil)
		}))
		return store
	}

	t.Run("read error", func(t *testing.T) {
		store := corruptConsensusStore(t)
		_, err := store.ReadLastConsensusSnapshot()
		require.Error(t, err)
	})

	t.Run("write propagates head read error", func(t *testing.T) {
		store := corruptConsensusStore(t)
		err := store.WriteConsensusSnapshot(genesisSnap.Snapshot, genesis, nil)
		require.Error(t, err)
	})
}
