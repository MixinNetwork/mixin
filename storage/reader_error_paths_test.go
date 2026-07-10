package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestGenesisReaderAndWriterErrors(t *testing.T) {
	node := crypto.Blake3Hash([]byte("genesis error node"))
	baseTx := common.NewTransactionV5(common.XINAssetId)
	baseTx.Inputs = []*common.Input{{Genesis: []byte("genesis error")}}
	baseTx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
	base := baseTx.AsVersioned()
	baseSnap := snapshotWithTopoForTx(node, 0, 0, 1, base)

	t.Run("missing topology target", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphTopologyKey(0), []byte("missing snapshot key"))
		}))
		loaded, err := store.CheckGenesisLoad([]*common.SnapshotWithTopologicalOrder{baseSnap})
		require.True(t, loaded)
		require.Error(t, err)
	})

	t.Run("malformed snapshot", func(t *testing.T) {
		store := newTestBadgerStore(t)
		target := []byte("malformed genesis snapshot key")
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphTopologyKey(0), target); err != nil {
				return err
			}
			return txn.Set(target, []byte{0})
		}))
		loaded, err := store.CheckGenesisLoad([]*common.SnapshotWithTopologicalOrder{baseSnap})
		require.True(t, loaded)
		require.Error(t, err)
	})

	t.Run("asset metadata", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphAssetInfoKey(common.XINAssetId), []byte("{"))
		}))
		err := store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{baseSnap}, []*common.VersionedTransaction{base})
		require.Error(t, err)
	})

	t.Run("transaction asset mismatch", func(t *testing.T) {
		store := newTestBadgerStore(t)
		assetID := crypto.Blake3Hash([]byte("genesis deposit asset"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return writeAssetInfo(txn, assetID, &common.Asset{Chain: common.BitcoinAssetId, AssetKey: "stored"})
		}))
		depositTx := common.NewTransactionV5(assetID)
		depositTx.Inputs = []*common.Input{{Deposit: &common.DepositData{
			Chain:       common.BitcoinAssetId,
			AssetKey:    "incoming",
			Transaction: "genesis deposit",
			Amount:      common.NewInteger(1),
		}}}
		depositTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
		ver := depositTx.AsVersioned()
		snap := snapshotWithTopoForTx(node, 0, 0, 1, ver)
		err := store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{snap}, []*common.VersionedTransaction{ver})
		require.ErrorContains(t, err, "invalid asset info")
	})

	t.Run("snapshot finalization", func(t *testing.T) {
		store := newTestBadgerStore(t)
		ghost := seededPublicKey(230)
		owner := crypto.Blake3Hash([]byte("genesis ghost owner"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphGhostKey(ghost), owner[:])
		}))
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: []byte("genesis ghost conflict")}}
		tx.Outputs = []*common.Output{{
			Type:   common.OutputTypeScript,
			Amount: common.NewInteger(1),
			Keys:   []*crypto.Key{&ghost},
			Script: common.NewThresholdScript(1),
		}}
		ver := tx.AsVersioned()
		snap := snapshotWithTopoForTx(node, 0, 0, 1, ver)
		err := store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{snap}, []*common.VersionedTransaction{ver})
		require.Error(t, err)
	})

	t.Run("consensus head", func(t *testing.T) {
		store := newTestBadgerStore(t)
		missing := crypto.Blake3Hash([]byte("genesis missing consensus head"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := txn.Set(graphSnapTopologyKey(missing), graphTopologyKey(99)); err != nil {
				return err
			}
			return txn.Set(graphConsensusSnapshotKey(100, missing), nil)
		}))
		err := store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{baseSnap}, []*common.VersionedTransaction{base})
		require.Error(t, err)
	})
}

func TestTopologyAndSnapshotWriteErrorPaths(t *testing.T) {
	node := crypto.Blake3Hash([]byte("topology error node"))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte("topology error")}}
	tx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	ver := tx.AsVersioned()
	snap := snapshotWithTopoForTx(node, 0, 1, 1, ver)

	t.Run("topology target", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphTopologyKey(1), []byte("missing topology target"))
		}))
		_, err := store.ReadSnapshotsSinceTopology(1, 1)
		require.Error(t, err)
		_, _, err = store.ReadSnapshotWithTransactionsSinceTopology(1, 1)
		require.Error(t, err)
		require.Panics(t, func() { store.LastSnapshot() })
	})

	t.Run("last snapshot transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			key := graphSnapshotKey(node, 0, snap.PayloadHash())
			if err := txn.Set(key, snap.VersionedMarshal()); err != nil {
				return err
			}
			if err := txn.Set(graphTopologyKey(1), key); err != nil {
				return err
			}
			return txn.Set(graphTransactionKey(ver.PayloadHash()), []byte{0})
		}))
		require.Panics(t, func() { store.LastSnapshot() })
	})

	t.Run("round references", func(t *testing.T) {
		store := newTestBadgerStore(t)
		self := crypto.Blake3Hash([]byte("self reference"))
		external := crypto.Blake3Hash([]byte("external reference"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
				return err
			}
			if err := writeTransaction(txn, ver); err != nil {
				return err
			}
			return writeRound(txn, node, &common.Round{
				Hash:       node,
				NodeId:     node,
				Number:     1,
				References: &common.RoundLink{Self: self, External: external},
			})
		}))
		bad := snapshotWithTopoForTx(node, 1, 1, 1, ver)
		bad.References = &common.RoundLink{Self: crypto.Blake3Hash([]byte("other self")), External: external}
		require.Panics(t, func() { _ = store.WriteSnapshot(bad, nil) })
	})

	t.Run("unique transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
				return err
			}
			return writeRound(txn, node, &common.Round{Hash: node, NodeId: node, References: &common.RoundLink{}})
		}))
		require.NoError(t, store.WriteTransaction(ver))
		require.NoError(t, store.WriteSnapshot(snap, nil))
		second := snapshotWithTopoForTx(node, 0, 2, 2, ver)
		require.Panics(t, func() { _ = store.WriteSnapshot(second, nil) })
	})

	t.Run("malformed transaction", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphTransactionKey(ver.PayloadHash()), []byte{0})
		}))
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return writeSnapshot(txn, snap)
		})
		require.Error(t, err)
	})

	t.Run("finalization output", func(t *testing.T) {
		store := newTestBadgerStore(t)
		ghost := seededPublicKey(231)
		owner := crypto.Blake3Hash([]byte("snapshot ghost owner"))
		ghostTx := common.NewTransactionV5(common.XINAssetId)
		ghostTx.Inputs = []*common.Input{{Genesis: []byte("snapshot ghost conflict")}}
		ghostTx.Outputs = []*common.Output{{
			Type:   common.OutputTypeScript,
			Amount: common.NewInteger(1),
			Keys:   []*crypto.Key{&ghost},
			Script: common.NewThresholdScript(1),
		}}
		ghostVer := ghostTx.AsVersioned()
		ghostSnap := snapshotWithTopoForTx(node, 0, 1, 1, ghostVer)
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
				return err
			}
			if err := writeRound(txn, node, &common.Round{Hash: node, NodeId: node, References: &common.RoundLink{}}); err != nil {
				return err
			}
			if err := writeTransaction(txn, ghostVer); err != nil {
				return err
			}
			return txn.Set(graphGhostKey(ghost), owner[:])
		}))
		err := store.WriteSnapshot(ghostSnap, nil)
		require.Error(t, err)
	})
}

func TestMintWithdrawalAndClosedReaderErrors(t *testing.T) {
	t.Run("unfinalized mint", func(t *testing.T) {
		store := newTestBadgerStore(t)
		mint := &common.MintData{Group: "UNIVERSAL", Batch: 10, Amount: common.NewInteger(1)}
		tx := common.NewTransactionV5(common.XINAssetId)
		tx.AddUniversalMintInput(mint.Batch, mint.Amount)
		ver := tx.AsVersioned()
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeMintDistribution(txn, mint, ver.PayloadHash()); err != nil {
				return err
			}
			return writeTransaction(txn, ver)
		}))
		mints, txs, err := store.ReadMintDistributions(0, 10)
		require.NoError(t, err)
		require.Empty(t, mints)
		require.Empty(t, txs)
	})

	t.Run("withdrawal records", func(t *testing.T) {
		store := newTestBadgerStore(t)
		base := crypto.Blake3Hash([]byte("malformed withdrawal"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphWithdrawalClaimKey(base), []byte{1})
		}))
		require.Panics(t, func() { _, _, _ = store.ReadWithdrawalClaim(base) })

		store = newTestBadgerStore(t)
		claim := crypto.Blake3Hash([]byte("withdrawal claim"))
		require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return txn.Set(graphTransactionKey(base), []byte{0})
		}))
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			return writeWithdrawalClaim(txn, base, claim)
		})
		require.Error(t, err)
	})

	t.Run("closed database", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		id := crypto.Blake3Hash([]byte("closed reader"))
		deposit := &common.DepositData{Chain: common.BitcoinAssetId, AssetKey: "btc", Transaction: "closed"}

		_, err := store.ReadUTXOKeys(id, 0)
		require.Error(t, err)
		_, err = store.ReadGhostKeyLock(seededPublicKey(232))
		require.Error(t, err)
		_, err = store.ReadDepositLock(deposit)
		require.Error(t, err)
		_, _, err = store.ReadWithdrawalClaim(id)
		require.Error(t, err)
		_, err = store.ListWorkOffsets([]crypto.Hash{id})
		require.Error(t, err)
		_, err = store.ListNodeWorks([]crypto.Hash{id}, 0)
		require.Error(t, err)
		_, err = store.ListAggregatedRoundSpaceCheckpoints([]crypto.Hash{id})
		require.Error(t, err)
	})
}
