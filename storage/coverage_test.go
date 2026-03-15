package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestCacheTransactionLifecycle(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)

	tx1 := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	tx2 := common.NewTransactionV5(common.XINAssetId)
	tx2.Extra = []byte{1}
	ver2 := tx2.AsVersioned()

	got, err := store.CacheGetTransaction(tx1.PayloadHash())
	require.Nil(err)
	require.Nil(got)

	err = store.CachePutTransaction(tx1)
	require.Nil(err)
	err = store.CachePutTransaction(ver2)
	require.Nil(err)
	err = store.CachePutTransaction(tx1)
	require.Nil(err)

	got, err = store.CacheGetTransaction(tx1.PayloadHash())
	require.Nil(err)
	require.Equal(tx1.PayloadHash(), got.PayloadHash())

	retrieved, err := store.CacheRetrieveTransactions(10)
	require.Nil(err)
	require.Len(retrieved, 2)

	hashes := map[string]bool{}
	for _, ver := range retrieved {
		hashes[ver.PayloadHash().String()] = true
	}
	require.True(hashes[tx1.PayloadHash().String()])
	require.True(hashes[ver2.PayloadHash().String()])

	retrieved, err = store.CacheRetrieveTransactions(10)
	require.Nil(err)
	require.Empty(retrieved)

	err = store.CacheRemoveTransactions([]crypto.Hash{tx1.PayloadHash(), ver2.PayloadHash()})
	require.Nil(err)

	got, err = store.CacheGetTransaction(tx1.PayloadHash())
	require.Nil(err)
	require.Nil(got)

	require.Equal(uint64(9), graphMintBatch(graphMintKey(9)))
	require.NotEmpty(cacheTransactionCacheKey(tx1.PayloadHash()))
	require.NotEmpty(cacheTransactionQueueKey(1, tx1.PayloadHash()))
	require.NotEmpty(cacheTransactionOrderKey(tx1.PayloadHash()))
}

func TestMintAndAssetHelpers(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	assetID := crypto.Blake3Hash([]byte("asset-id"))
	asset := &common.Asset{
		Chain:    common.BitcoinAssetId,
		AssetKey: "btc",
	}

	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		old, err := readAssetInfo(txn, assetID)
		require.Nil(err)
		require.Nil(old)

		err = writeAssetInfo(txn, assetID, asset)
		require.Nil(err)
		err = verifyAssetInfo(txn, assetID, asset)
		require.Nil(err)
		err = verifyAssetInfo(txn, assetID, &common.Asset{
			Chain:    common.BitcoinAssetId,
			AssetKey: "other",
		})
		require.ErrorContains(err, "invalid asset info")

		tx := common.NewTransactionV5(assetID)
		tx.Inputs = []*common.Input{{Genesis: []byte("genesis")}}
		tx.Outputs = []*common.Output{{
			Type:   common.OutputTypeScript,
			Amount: common.NewInteger(5),
		}}
		err = writeTotalInAsset(txn, tx.AsVersioned())
		require.Nil(err)

		total, err := readTotalInAsset(txn, assetID)
		require.Nil(err)
		require.Equal("5.00000000", total.String())
		return nil
	})
	require.Nil(err)

	readAsset, balance, err := store.ReadAssetWithBalance(assetID)
	require.Nil(err)
	require.Equal(asset.AssetKey, readAsset.AssetKey)
	require.Equal("5.00000000", balance.String())

	_, _, err = store.ReadMintDistributions(0, 501)
	require.ErrorContains(err, "maximum is 500")

	mint := &common.MintData{
		Group:  "UNIVERSAL",
		Batch:  7,
		Amount: common.NewInteger(3),
	}
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddUniversalMintInput(mint.Batch, mint.Amount)
	ver := tx.AsVersioned()

	last, err := store.ReadLastMintDistribution(^uint64(0))
	require.Nil(err)
	require.Nil(last)

	err = store.LockMintInput(mint, ver.PayloadHash(), false)
	require.Nil(err)

	last, err = store.ReadLastMintDistribution(^uint64(0))
	require.Nil(err)
	require.Nil(last)

	mints, txs, err := store.ReadMintDistributions(0, 10)
	require.Nil(err)
	require.Empty(mints)
	require.Empty(txs)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeTransaction(txn, ver)
		if err != nil {
			return err
		}
		final := crypto.Blake3Hash([]byte("mint-finalization"))
		return txn.Set(graphFinalizationKey(ver.PayloadHash()), final[:])
	})
	require.Nil(err)

	last, err = store.ReadLastMintDistribution(^uint64(0))
	require.Nil(err)
	require.NotNil(last)
	require.Equal(ver.PayloadHash(), last.Transaction)

	mints, txs, err = store.ReadMintDistributions(0, 10)
	require.Nil(err)
	require.Len(mints, 1)
	require.Len(txs, 1)
	require.Equal(ver.PayloadHash(), txs[0].PayloadHash())

	err = store.LockMintInput(mint, ver.PayloadHash(), false)
	require.Nil(err)

	other := common.NewTransactionV5(common.XINAssetId)
	other.Extra = []byte{9}
	other.AddUniversalMintInput(mint.Batch, mint.Amount)
	err = store.LockMintInput(mint, other.AsVersioned().PayloadHash(), false)
	require.ErrorContains(err, "mint locked")

	mint2 := &common.MintData{
		Group:  "UNIVERSAL",
		Batch:  8,
		Amount: common.NewInteger(4),
	}
	err = store.LockMintInput(mint2, crypto.Blake3Hash([]byte("mint-2")), false)
	require.Nil(err)
	err = store.LockMintInput(mint2, crypto.Blake3Hash([]byte("mint-2-replaced")), true)
	require.Nil(err)

	err = store.snapshotsDB.View(func(txn *badger.Txn) error {
		dist, err := readMintInput(txn, mint2)
		require.Nil(err)
		require.Equal(crypto.Blake3Hash([]byte("mint-2-replaced")), dist.Transaction)
		return nil
	})
	require.Nil(err)
}

func TestAssetTotalAndErrorBranches(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	assetID := crypto.Blake3Hash([]byte("asset-edges"))
	asset := &common.Asset{
		Chain:    common.BitcoinAssetId,
		AssetKey: "btc",
	}

	readAsset, balance, err := store.ReadAssetWithBalance(assetID)
	require.Nil(err)
	require.Nil(readAsset)
	require.Equal(0, balance.Cmp(common.Zero))

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		missingTx := common.NewTransactionV5(assetID)
		missingTx.Inputs = []*common.Input{{Genesis: []byte("missing")}}
		missingTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
		require.Panics(func() {
			_ = writeTotalInAsset(txn, missingTx.AsVersioned())
		})

		badAssetID := crypto.Blake3Hash([]byte("bad-asset"))
		err := txn.Set(graphAssetInfoKey(badAssetID), []byte("{"))
		require.Nil(err)
		_, err = readAssetInfo(txn, badAssetID)
		require.Error(err)

		err = txn.Set(graphAssetTotalKey(assetID), []byte("bad"))
		require.Nil(err)
		require.Panics(func() {
			_, _ = readTotalInAsset(txn, assetID)
		})
		err = txn.Delete(graphAssetTotalKey(assetID))
		require.Nil(err)

		err = writeAssetInfo(txn, assetID, asset)
		require.Nil(err)

		genesisTx := common.NewTransactionV5(assetID)
		genesisTx.Inputs = []*common.Input{{Genesis: []byte("genesis")}}
		genesisTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(5)}}
		err = writeTotalInAsset(txn, genesisTx.AsVersioned())
		require.Nil(err)

		depositTx := common.NewTransactionV5(assetID)
		depositTx.Inputs = []*common.Input{{
			Deposit: &common.DepositData{
				Chain:       common.BitcoinAssetId,
				AssetKey:    "btc",
				Transaction: "deposit-edge",
				Index:       7,
				Amount:      common.NewInteger(2),
			},
		}}
		err = writeTotalInAsset(txn, depositTx.AsVersioned())
		require.Nil(err)

		mintTx := common.NewTransactionV5(assetID)
		mintTx.AddUniversalMintInput(5, common.NewInteger(3))
		err = writeTotalInAsset(txn, mintTx.AsVersioned())
		require.Nil(err)

		withdrawalTx := common.NewTransactionV5(assetID)
		withdrawalTx.Outputs = []*common.Output{{
			Type:   common.OutputTypeWithdrawalSubmit,
			Amount: common.NewInteger(4),
		}}
		err = writeTotalInAsset(txn, withdrawalTx.AsVersioned())
		require.Nil(err)

		scriptTx := common.NewTransactionV5(assetID)
		scriptTx.Inputs = []*common.Input{{Hash: crypto.Blake3Hash([]byte("plain-input")), Index: 0}}
		scriptTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(99)}}
		err = writeTotalInAsset(txn, scriptTx.AsVersioned())
		require.Nil(err)

		total, err := readTotalInAsset(txn, assetID)
		require.Nil(err)
		require.Equal("6.00000000", total.String())
		return nil
	})
	require.Nil(err)

	readAsset, balance, err = store.ReadAssetWithBalance(assetID)
	require.Nil(err)
	require.Equal(asset.AssetKey, readAsset.AssetKey)
	require.Equal("6.00000000", balance.String())
}

func TestDepositUTXOAndGhostLockHelpers(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	deposit := &common.DepositData{
		Chain:       common.BitcoinAssetId,
		AssetKey:    "btc",
		Transaction: "deposit-tx",
		Index:       1,
		Amount:      common.NewInteger(2),
	}
	lock1 := crypto.Blake3Hash([]byte("deposit-lock-1"))
	lock2 := crypto.Blake3Hash([]byte("deposit-lock-2"))

	readLock, err := store.ReadDepositLock(deposit)
	require.Nil(err)
	require.False(readLock.HasValue())

	err = store.LockDepositInput(deposit, lock1, false)
	require.Nil(err)
	readLock, err = store.ReadDepositLock(deposit)
	require.Nil(err)
	require.Equal(lock1, readLock)

	err = store.LockDepositInput(deposit, lock1, false)
	require.Nil(err)
	err = store.LockDepositInput(deposit, lock2, false)
	require.ErrorContains(err, "deposit locked")
	err = store.LockDepositInput(deposit, lock2, true)
	require.Nil(err)

	public1 := seededPublicKey(31)
	public2 := seededPublicKey(32)
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Outputs = []*common.Output{{
		Type:   common.OutputTypeScript,
		Amount: common.NewInteger(9),
		Keys:   []*crypto.Key{&public1},
		Script: common.NewThresholdScript(1),
	}}
	utxo := tx.AsVersioned().UnspentOutputs()[0]

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphUtxoKey(utxo.Hash, utxo.Index), utxo.Marshal())
	})
	require.Nil(err)

	keys, err := store.ReadUTXOKeys(utxo.Hash, utxo.Index)
	require.Nil(err)
	require.Len(keys.Keys, 1)

	readUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
	require.Nil(err)
	require.False(readUTXO.LockHash.HasValue())

	err = store.LockUTXOs([]*common.Input{{Hash: utxo.Hash, Index: utxo.Index}}, lock1, false)
	require.Nil(err)

	readUTXO, err = store.ReadUTXOLock(utxo.Hash, utxo.Index)
	require.Nil(err)
	require.Equal(lock1, readUTXO.LockHash)

	err = store.LockUTXOs([]*common.Input{{Hash: utxo.Hash, Index: utxo.Index}}, lock1, false)
	require.Nil(err)
	err = store.LockUTXOs([]*common.Input{{Hash: utxo.Hash, Index: utxo.Index}}, lock2, false)
	require.ErrorContains(err, "utxo locked")
	err = store.LockUTXOs([]*common.Input{{Hash: utxo.Hash, Index: utxo.Index}}, lock2, true)
	require.Nil(err)

	ghostLock, err := store.ReadGhostKeyLock(public1)
	require.Nil(err)
	require.Nil(ghostLock)

	err = store.LockGhostKeys([]*crypto.Key{&public1}, lock1, false)
	require.Nil(err)
	ghostLock, err = store.ReadGhostKeyLock(public1)
	require.Nil(err)
	require.Equal(lock1, *ghostLock)

	err = store.LockGhostKeys([]*crypto.Key{&public1, &public1}, lock1, false)
	require.ErrorContains(err, "duplicated ghost key")
	err = store.LockGhostKeys([]*crypto.Key{&public1}, lock2, false)
	require.ErrorContains(err, "locked for transaction")

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphGhostKey(public2), []byte{1, 2, 3})
	})
	require.Nil(err)
	err = store.snapshotsDB.View(func(txn *badger.Txn) error {
		return lockGhostKey(txn, &public2, lock1, false)
	})
	require.ErrorContains(err, "malformed lock")
}

func TestTopologyAndTransactionReaders(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	assetID := crypto.Blake3Hash([]byte("topology-asset"))
	nodeID := crypto.Blake3Hash([]byte("topology-node"))
	asset := &common.Asset{
		Chain:    common.BitcoinAssetId,
		AssetKey: "btc",
	}

	tx := common.NewTransactionV5(assetID)
	tx.Inputs = []*common.Input{{Genesis: []byte("genesis")}}
	tx.Outputs = []*common.Output{{
		Type:   common.OutputTypeScript,
		Amount: common.NewInteger(7),
	}}
	ver := tx.AsVersioned()

	snap := &common.SnapshotWithTopologicalOrder{
		Snapshot: &common.Snapshot{
			Version:      common.SnapshotVersionCommonEncoding,
			NodeId:       nodeID,
			RoundNumber:  1,
			Timestamp:    2,
			Transactions: []crypto.Hash{ver.PayloadHash()},
		},
		TopologicalOrder: 1,
	}

	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeAssetInfo(txn, assetID, asset)
		if err != nil {
			return err
		}
		err = writeTransaction(txn, ver)
		if err != nil {
			return err
		}
		return writeSnapshot(txn, snap, ver)
	})
	require.Nil(err)

	readVer, final, err := store.ReadTransaction(ver.PayloadHash())
	require.Nil(err)
	require.Equal(ver.PayloadHash(), readVer.PayloadHash())
	require.Len(final, 64)

	readSnap, err := store.ReadSnapshot(snap.PayloadHash())
	require.Nil(err)
	require.Equal(snap.PayloadHash(), readSnap.PayloadHash())
	require.Equal(uint64(1), readSnap.TopologicalOrder)

	snaps, txs, err := store.ReadSnapshotWithTransactionsSinceTopology(1, 10)
	require.Nil(err)
	require.Len(snaps, 1)
	require.Len(txs, 1)
	require.Equal(ver.PayloadHash(), txs[0].PayloadHash())

	_, _, err = store.ReadSnapshotWithTransactionsSinceTopology(1, 501)
	require.ErrorContains(err, "maximum is 500")

	snaps, err = store.ReadSnapshotsSinceTopology(1, 10)
	require.Nil(err)
	require.Len(snaps, 1)

	nodeSnaps, err := store.ReadSnapshotsForNodeRound(nodeID, 1)
	require.Nil(err)
	require.Len(nodeSnaps, 1)

	lastSnap, lastTx := store.LastSnapshot()
	require.Equal(snap.PayloadHash(), lastSnap.PayloadHash())
	require.Equal(ver.PayloadHash(), lastTx.PayloadHash())

	removed, err := store.RemoveGraphEntries(graphPrefixUnique)
	require.Nil(err)
	require.Equal(1, removed)
}

func TestNodeRoundWorkAndSpaceHelpers(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	signer1 := seededAddress(41)
	payee1 := seededAddress(42)
	signer2 := seededAddress(43)
	payee2 := seededAddress(44)
	signer3 := seededAddress(45)
	payee3 := seededAddress(46)

	tx1 := crypto.Blake3Hash([]byte("node-pledge-1"))
	tx2 := crypto.Blake3Hash([]byte("node-cancel-1"))
	tx3 := crypto.Blake3Hash([]byte("node-accept-2"))
	tx4 := crypto.Blake3Hash([]byte("node-remove-2"))
	tx5 := crypto.Blake3Hash([]byte("node-pledge-3"))
	tx6 := crypto.Blake3Hash([]byte("node-accept-3"))

	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeNodePledge(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, tx1, 10)
		require.Nil(err)

		nodes := readAllNodes(txn, 9, false)
		require.Empty(nodes)

		nodes = readAllNodes(txn, 10, true)
		require.Len(nodes, 1)
		require.Equal(common.NodeStatePledging, nodes[0].State)

		err = writeNodeCancel(txn, signer1.PublicSpendKey, payee1.PublicSpendKey, tx2, 11)
		require.Nil(err)

		err = writeNodeAccept(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, tx3, 20, true)
		require.Nil(err)
		err = writeNodeRemove(txn, signer2.PublicSpendKey, payee2.PublicSpendKey, tx4, 21)
		require.Nil(err)

		err = writeNodePledge(txn, signer3.PublicSpendKey, payee3.PublicSpendKey, tx5, 30)
		require.Nil(err)
		err = writeNodeAccept(txn, signer3.PublicSpendKey, payee3.PublicSpendKey, tx6, 31, false)
		require.Nil(err)

		key := nodeStateQueueKey(signer1.PublicSpendKey, 10)
		gotSigner, ts := nodeSignerFromStateKey(key)
		require.Equal(uint64(10), ts)
		require.Equal(signer1.PublicSpendKey, gotSigner.PublicSpendKey)

		val := nodeEntryValue(payee1.PublicSpendKey, tx1, common.NodeStatePledging)
		require.Equal(payee1.PublicSpendKey, nodePayee(val).PublicSpendKey)
		require.Equal(tx1, nodeTransaction(val))
		require.Equal(common.NodeStatePledging, nodeState(val))

		return nil
	})
	require.Nil(err)

	nodes := store.ReadAllNodes(^uint64(0), false)
	require.Len(nodes, 3)
	require.Equal(common.NodeStateCancelled, nodes[0].State)
	require.Equal(common.NodeStateRemoved, nodes[1].State)
	require.Equal(common.NodeStateAccepted, nodes[2].State)

	pledgeVer := nodeOpTransaction(common.OutputTypeNodePledge)
	cancelVer := nodeOpTransaction(common.OutputTypeNodeCancel)
	scriptVer := common.NewTransactionV5(common.XINAssetId).AsVersioned()

	err = store.AddNodeOperation(scriptVer, 100, 10, false)
	require.ErrorContains(err, "invalid operation")
	err = store.AddNodeOperation(pledgeVer, 100, 10, false)
	require.Nil(err)
	err = store.AddNodeOperation(pledgeVer, 105, 10, false)
	require.Nil(err)
	err = store.AddNodeOperation(cancelVer, 105, 10, false)
	require.ErrorContains(err, "invalid operation lock")
	err = store.AddNodeOperation(cancelVer, 105, 10, true)
	require.Nil(err)

	err = store.snapshotsDB.View(func(txn *badger.Txn) error {
		op, hash, ts, err := readLastNodeOperation(txn)
		require.Nil(err)
		require.Equal("CANCEL", op)
		require.Equal(cancelVer.PayloadHash(), hash)
		require.Equal(uint64(105), ts)
		return nil
	})
	require.Nil(err)

	nodeID := crypto.Blake3Hash([]byte("round-node"))
	externalID := crypto.Blake3Hash([]byte("round-external"))
	selfFinal := crypto.Blake3Hash([]byte("round-self-final"))
	refs := &common.RoundLink{Self: selfFinal, External: externalID}

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}})
		require.Nil(err)
		err = writeRound(txn, externalID, &common.Round{Hash: externalID, NodeId: externalID, Number: 0, References: &common.RoundLink{}})
		require.Nil(err)
		return nil
	})
	require.Nil(err)

	link, err := store.ReadLink(nodeID, externalID)
	require.Nil(err)
	require.Equal(uint64(0), link)

	err = store.StartNewRound(nodeID, 1, refs, 77)
	require.Nil(err)

	headRound, err := store.ReadRound(nodeID)
	require.Nil(err)
	require.Equal(uint64(1), headRound.Number)
	require.Equal(refs.Self, headRound.References.Self)

	finalRound, err := store.ReadRound(selfFinal)
	require.Nil(err)
	require.Equal(uint64(0), finalRound.Number)
	require.Equal(uint64(77), finalRound.Timestamp)

	err = store.UpdateEmptyHeadRound(nodeID, 1, refs)
	require.Nil(err)

	worksNode := crypto.Blake3Hash([]byte("works-node"))
	otherNode := crypto.Blake3Hash([]byte("works-other"))
	ts := uint64(DAY_U64 + 10)
	works := []*common.SnapshotWork{
		{Hash: crypto.Blake3Hash([]byte("work-1")), Timestamp: ts, Signers: []crypto.Hash{worksNode, otherNode}},
		{Hash: crypto.Blake3Hash([]byte("work-2")), Timestamp: ts + 1, Signers: []crypto.Hash{worksNode}},
	}

	offset, err := store.ReadWorkOffset(worksNode)
	require.Nil(err)
	require.Equal(uint64(0), offset)

	err = store.WriteRoundWork(worksNode, 1, works, true)
	require.Nil(err)
	err = store.WriteRoundWork(worksNode, 1, works, true)
	require.Nil(err)
	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		for _, w := range works {
			snap := &common.SnapshotWithTopologicalOrder{
				Snapshot: &common.Snapshot{
					Version:     common.SnapshotVersionCommonEncoding,
					NodeId:      worksNode,
					RoundNumber: 1,
					Timestamp:   w.Timestamp,
					Hash:        w.Hash,
				},
			}
			err := writeSnapshotWork(txn, snap, w.Signers)
			if err != nil {
				return err
			}
		}
		return nil
	})
	require.Nil(err)

	offset, err = store.ReadWorkOffset(worksNode)
	require.Nil(err)
	require.Equal(uint64(1), offset)

	workItems, err := store.ReadSnapshotWorksForNodeRound(worksNode, 1)
	require.Nil(err)
	require.Len(workItems, 2)

	offsets, err := store.ListWorkOffsets([]crypto.Hash{worksNode, otherNode})
	require.Nil(err)
	require.Equal(uint64(1), offsets[worksNode])
	require.Equal(uint64(0), offsets[otherNode])

	day := uint32(ts / DAY_U64)
	nodeWorks, err := store.ListNodeWorks([]crypto.Hash{worksNode, otherNode}, day)
	require.Nil(err)
	require.Equal([2]uint64{2, 0}, nodeWorks[worksNode])
	require.Equal([2]uint64{0, 1}, nodeWorks[otherNode])

	err = store.WriteRoundWork(worksNode, 2, []*common.SnapshotWork{
		{Hash: crypto.Blake3Hash([]byte("work-3")), Timestamp: ts + DAY_U64},
	}, false)
	require.Nil(err)
	workItems, err = store.ReadSnapshotWorksForNodeRound(worksNode, 1)
	require.Nil(err)
	require.Empty(workItems)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphWorkOffsetKey(worksNode)
		off, hashes, err := graphReadWorkOffset(txn, key)
		require.Nil(err)
		require.Equal(uint64(2), off)
		require.Len(hashes, 1)

		err = graphWriteUint64(txn, graphWorkLeadKey(worksNode, day), 9)
		require.Nil(err)
		val, err := graphReadUint64(txn, graphWorkLeadKey(worksNode, day))
		require.Nil(err)
		require.Equal(uint64(9), val)

		return removeSnapshotWorksForRound(txn, worksNode, 2)
	})
	require.Nil(err)

	space := &common.RoundSpace{
		NodeId:   worksNode,
		Batch:    1,
		Round:    1,
		Duration: uint64(config.CheckpointDuration),
	}
	err = store.WriteRoundSpaceAndState(space)
	require.Nil(err)
	err = store.WriteRoundSpaceAndState(&common.RoundSpace{
		NodeId: worksNode,
		Batch:  2,
		Round:  2,
	})
	require.Nil(err)

	batch, round, err := store.ReadRoundSpaceCheckpoint(worksNode)
	require.Nil(err)
	require.Equal(uint64(2), batch)
	require.Equal(uint64(2), round)

	spaces, err := store.ListAggregatedRoundSpaceCheckpoints([]crypto.Hash{worksNode})
	require.Nil(err)
	require.Equal(uint64(2), spaces[worksNode].Batch)

	queue, err := store.ReadNodeRoundSpacesForBatch(worksNode, 1)
	require.Nil(err)
	require.Len(queue, 1)
	require.Equal(uint64(config.CheckpointDuration), queue[0].Duration)

	s1 := &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       nodeID,
		Timestamp:    5,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("compute-1"))},
		Hash:         crypto.Blake3Hash([]byte("compute-1-hash")),
	}}
	s2 := &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       nodeID,
		Timestamp:    6,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("compute-2"))},
		Hash:         crypto.Blake3Hash([]byte("compute-2-hash")),
	}}
	start, end, hash := computeRoundHash(nodeID, 7, []*common.SnapshotWithTopologicalOrder{s2, s1})
	_, _, expected := common.ComputeRoundHash(nodeID, 7, []*common.Snapshot{s1.Snapshot, s2.Snapshot})
	require.Equal(uint64(5), start)
	require.Equal(uint64(6), end)
	require.Equal(expected, hash)
}

func TestCustodianAndValidationHelpers(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	networkID := crypto.Blake3Hash([]byte("custodian-network"))
	custodianVer1, extra1 := buildCustodianUpdateTransaction(70, networkID)
	custodianVer2, extra2 := buildCustodianUpdateTransaction(100, networkID)

	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeTransaction(txn, custodianVer1)
		if err != nil {
			return err
		}
		utxo1 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: custodianVer1.PayloadHash()}}}
		err = writeCustodianNodes(txn, 100, utxo1, extra1, true)
		if err != nil {
			return err
		}
		err = writeCustodianNodes(txn, 100, utxo1, extra1, true)
		if err != nil {
			return err
		}

		err = writeTransaction(txn, custodianVer2)
		if err != nil {
			return err
		}
		utxo2 := &common.UTXOWithLock{UTXO: common.UTXO{Input: common.Input{Hash: custodianVer2.PayloadHash()}}}
		return writeCustodianNodes(txn, 200, utxo2, extra2, false)
	})
	require.Nil(err)

	updates, err := store.ListCustodianUpdates()
	require.Nil(err)
	require.Len(updates, 2)
	require.Equal(custodianVer1.PayloadHash(), updates[0].Transaction)
	require.Equal(custodianVer2.PayloadHash(), updates[1].Transaction)

	cur, err := store.ReadCustodian(150)
	require.Nil(err)
	require.Equal(custodianVer1.PayloadHash(), cur.Transaction)
	cur, err = store.ReadCustodian(250)
	require.Nil(err)
	require.Equal(custodianVer2.PayloadHash(), cur.Transaction)
	require.Equal(uint64(200), graphCustodianAccountTimestamp(graphCustodianUpdateKey(200)))

	validationStore := newTestBadgerStore(t)
	signer := seededNodeAddress(120)
	payee := seededNodeAddress(121)
	nodeID := signer.Hash().ForNetwork(networkID)
	ver := common.NewTransactionV5(common.XINAssetId)
	ver.Inputs = []*common.Input{{Genesis: []byte("genesis")}}
	ver.Outputs = []*common.Output{{
		Type:   common.OutputTypeScript,
		Amount: common.NewInteger(1),
	}}
	signed := ver.AsVersioned()
	snap := &common.SnapshotWithTopologicalOrder{
		Snapshot: &common.Snapshot{
			Version:      common.SnapshotVersionCommonEncoding,
			NodeId:       nodeID,
			RoundNumber:  0,
			Timestamp:    2,
			Transactions: []crypto.Hash{signed.PayloadHash()},
		},
		TopologicalOrder: 1,
	}
	snap.Hash = snap.PayloadHash()
	_, _, roundHash := computeRoundHash(nodeID, 0, []*common.SnapshotWithTopologicalOrder{snap})

	err = validationStore.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeAssetInfo(txn, common.XINAssetId, &common.Asset{
			Chain:    common.BitcoinAssetId,
			AssetKey: "btc",
		})
		if err != nil {
			return err
		}
		err = writeTransaction(txn, signed)
		if err != nil {
			return err
		}
		err = writeNodeAccept(txn, signer.PublicSpendKey, payee.PublicSpendKey, crypto.Blake3Hash([]byte("node-state")), 1, true)
		if err != nil {
			return err
		}
		err = writeSnapshot(txn, snap, signed)
		if err != nil {
			return err
		}
		err = writeRound(txn, roundHash, &common.Round{
			Hash:      roundHash,
			NodeId:    nodeID,
			Number:    0,
			Timestamp: snap.Timestamp,
		})
		if err != nil {
			return err
		}
		return writeRound(txn, nodeID, &common.Round{
			Hash:      nodeID,
			NodeId:    nodeID,
			Number:    1,
			Timestamp: snap.Timestamp,
		})
	})
	require.Nil(err)

	validationNodes := validationStore.ReadAllNodes(^uint64(0), false)
	require.Len(validationNodes, 1)
	require.Equal(nodeID, validationNodes[0].IdForNetwork(networkID))
	head, err := validationStore.ReadRound(nodeID)
	require.Nil(err)
	require.NotNil(head)
	roundSnapshots, err := validationStore.ReadSnapshotsForNodeRound(nodeID, 0)
	require.Nil(err)
	require.Len(roundSnapshots, 1)

	total, invalid, err := validationStore.ValidateGraphEntries(networkID, 10)
	require.Nil(err)
	require.Equal(0, invalid)
	require.GreaterOrEqual(total, 0)

	total, invalid, err = validationStore.validateSnapshotEntriesForNode(nodeID, 10)
	require.Nil(err)
	require.Equal(1, total)
	require.Equal(0, invalid)
}

func TestTransactionWithdrawalAndConsensusEdges(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	nodeID := crypto.Blake3Hash([]byte("transaction-node"))
	signer := seededAddress(160)
	payee := seededAddress(161)
	baseTx := common.NewTransactionV5(common.XINAssetId)
	baseTx.Inputs = []*common.Input{{Genesis: []byte("base-genesis")}}
	baseTx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
	baseTx.Extra = append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)
	baseVer := baseTx.AsVersioned()
	baseSnap := snapshotWithTopoForTx(nodeID, 0, 1, 1, baseVer)

	claimTx := common.NewTransactionV5(common.XINAssetId)
	claimTx.Inputs = []*common.Input{{Genesis: []byte("claim-genesis")}}
	claimTx.Outputs = []*common.Output{{Type: common.OutputTypeWithdrawalClaim, Amount: common.NewInteger(1)}}
	claimTx.References = []crypto.Hash{baseVer.PayloadHash()}
	claimVer := claimTx.AsVersioned()
	claimSnap := snapshotWithTopoForTx(nodeID, 0, 2, 2, claimVer)

	depositTx := common.NewTransactionV5(common.XINAssetId)
	depositTx.Inputs = []*common.Input{{
		Deposit: &common.DepositData{
			Chain:       common.XINAsset.Chain,
			AssetKey:    common.XINAsset.AssetKey,
			Transaction: "deposit-finalize",
			Index:       9,
			Amount:      common.NewInteger(2),
		},
	}}
	depositTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(2)}}
	depositVer := depositTx.AsVersioned()
	depositSnap := snapshotWithTopoForTx(nodeID, 0, 3, 3, depositVer)

	mintTx := common.NewTransactionV5(common.XINAssetId)
	mintTx.AddUniversalMintInput(11, common.NewInteger(3))
	mintTx.References = []crypto.Hash{baseVer.PayloadHash()}
	mintTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(3)}}
	mintVer := mintTx.AsVersioned()
	mintSnap := snapshotWithTopoForTx(nodeID, 0, 4, 4, mintVer)

	readClaim, readFinal, err := store.ReadWithdrawalClaim(baseVer.PayloadHash())
	require.Nil(err)
	require.Nil(readClaim)
	require.Empty(readFinal)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset)
		if err != nil {
			return err
		}
		err = writeTransaction(txn, baseVer)
		if err != nil {
			return err
		}

		readVer, final, err := readTransactionAndFinalization(txn, baseVer.PayloadHash())
		require.Nil(err)
		require.Equal(baseVer.PayloadHash(), readVer.PayloadHash())
		require.Empty(final)

		err = pruneTransaction(txn, baseVer.PayloadHash())
		require.Nil(err)
		err = writeTransaction(txn, baseVer)
		require.Nil(err)

		err = txn.Set(graphFinalizationKey(baseVer.PayloadHash()), []byte{1})
		require.Nil(err)
		require.Panics(func() {
			_, _, _ = readTransactionAndFinalization(txn, baseVer.PayloadHash())
		})
		err = txn.Delete(graphFinalizationKey(baseVer.PayloadHash()))
		require.Nil(err)

		require.Panics(func() {
			_ = writeWithdrawalClaim(txn, crypto.Blake3Hash([]byte("missing-withdrawal")), claimVer.PayloadHash())
		})

		err = writeSnapshot(txn, baseSnap, baseVer)
		require.Nil(err)
		require.Panics(func() {
			_ = writeTopology(txn, baseSnap)
		})
		err = pruneTransaction(txn, baseVer.PayloadHash())
		require.ErrorContains(err, "prune finalized transaction")

		err = writeTransaction(txn, claimVer)
		require.Nil(err)
		err = writeSnapshot(txn, claimSnap, claimVer)
		require.Nil(err)

		err = writeTransaction(txn, depositVer)
		require.Nil(err)
		err = finalizeTransaction(txn, depositVer, depositSnap)
		require.Nil(err)

		err = writeTransaction(txn, mintVer)
		require.Nil(err)
		err = writeSnapshot(txn, mintSnap, mintVer)
		require.Nil(err)

		err = writeConsensusSnapshot(txn, baseSnap.Snapshot, baseVer, nil)
		require.Nil(err)
		last, err := readLastConsensusSnapshot(txn)
		require.Nil(err)
		require.Equal(baseSnap.PayloadHash(), last.PayloadHash())

		err = writeConsensusSnapshot(txn, mintSnap.Snapshot, mintVer, nil)
		require.Nil(err)
		last, err = readLastConsensusSnapshot(txn)
		require.Nil(err)
		require.Equal(mintSnap.PayloadHash(), last.PayloadHash())

		err = writeConsensusSnapshot(txn, mintSnap.Snapshot, mintVer, nil)
		require.Nil(err)
		return nil
	})
	require.Nil(err)

	readClaim, readFinal, err = store.ReadWithdrawalClaim(baseVer.PayloadHash())
	require.Nil(err)
	require.Equal(claimVer.PayloadHash(), readClaim.PayloadHash())
	require.Len(readFinal, 64)

	lastSnap, lastTx := store.LastSnapshot()
	require.Equal(mintSnap.PayloadHash(), lastSnap.PayloadHash())
	require.Equal(mintVer.PayloadHash(), lastTx.PayloadHash())
}

func TestPublicTransactionAndSnapshotWorkflows(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeAssetInfo(txn, common.XINAssetId, common.XINAsset)
	})
	require.Nil(err)

	deposit := &common.DepositData{
		Chain:       common.XINAsset.Chain,
		AssetKey:    common.XINAsset.AssetKey,
		Transaction: "public-deposit",
		Index:       1,
		Amount:      common.NewInteger(1),
	}
	depositTx := common.NewTransactionV5(common.XINAssetId)
	depositTx.Inputs = []*common.Input{{Deposit: deposit}}
	depositTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	depositVer := depositTx.AsVersioned()
	err = store.LockDepositInput(deposit, depositVer.PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(depositVer)
	require.Nil(err)

	depositMismatch := common.NewTransactionV5(common.XINAssetId)
	depositMismatch.Inputs = []*common.Input{{Deposit: deposit}}
	depositMismatch.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	depositMismatch.Extra = []byte{1}
	require.Panics(func() {
		_ = store.WriteTransaction(depositMismatch.AsVersioned())
	})

	mint := &common.MintData{Group: "UNIVERSAL", Batch: 31, Amount: common.NewInteger(1)}
	mintTx := common.NewTransactionV5(common.XINAssetId)
	mintTx.AddUniversalMintInput(mint.Batch, mint.Amount)
	mintTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	mintVer := mintTx.AsVersioned()
	err = store.LockMintInput(mint, mintVer.PayloadHash(), false)
	require.Nil(err)
	err = store.WriteTransaction(mintVer)
	require.Nil(err)

	mintMismatch := common.NewTransactionV5(common.XINAssetId)
	mintMismatch.AddUniversalMintInput(mint.Batch, mint.Amount)
	mintMismatch.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	mintMismatch.Extra = []byte{2}
	require.Panics(func() {
		_ = store.WriteTransaction(mintMismatch.AsVersioned())
	})

	prevTx := common.NewTransactionV5(common.XINAssetId)
	prevTx.Inputs = []*common.Input{{Genesis: []byte("public-prev")}}
	prevTx.Outputs = []*common.Output{{
		Type:   common.OutputTypeScript,
		Amount: common.NewInteger(1),
		Keys:   []*crypto.Key{func() *crypto.Key { k := seededPublicKey(200); return &k }()},
		Script: common.NewThresholdScript(1),
	}}
	prevUTXO := prevTx.AsVersioned().UnspentOutputs()[0]
	spendTx := common.NewTransactionV5(common.XINAssetId)
	spendTx.Inputs = []*common.Input{{Hash: prevUTXO.Hash, Index: prevUTXO.Index}}
	spendTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	spendVer := spendTx.AsVersioned()
	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		prevUTXO.LockHash = spendVer.PayloadHash()
		return txn.Set(graphUtxoKey(prevUTXO.Hash, prevUTXO.Index), prevUTXO.Marshal())
	})
	require.Nil(err)
	err = store.WriteTransaction(spendVer)
	require.Nil(err)

	spendMismatch := common.NewTransactionV5(common.XINAssetId)
	spendMismatch.Inputs = []*common.Input{{Hash: prevUTXO.Hash, Index: prevUTXO.Index}}
	spendMismatch.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	spendMismatch.Extra = []byte{3}
	require.Panics(func() {
		_ = store.WriteTransaction(spendMismatch.AsVersioned())
	})

	nodeID := crypto.Blake3Hash([]byte("public-snapshot-node"))
	snapshotTx := common.NewTransactionV5(common.XINAssetId)
	snapshotTx.Inputs = []*common.Input{{Genesis: []byte("public-snapshot")}}
	snapshotTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	snapshotVer := snapshotTx.AsVersioned()
	err = store.WriteTransaction(snapshotVer)
	require.Nil(err)
	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}})
	})
	require.Nil(err)

	snap := snapshotWithTopoForTx(nodeID, 0, 10, 10, snapshotVer)
	err = store.WriteSnapshot(snap, []crypto.Hash{nodeID})
	require.Nil(err)
	readSnap, err := store.ReadSnapshot(snap.PayloadHash())
	require.Nil(err)
	require.Equal(snap.PayloadHash(), readSnap.PayloadHash())
	require.Panics(func() {
		_ = store.WriteSnapshot(snap, nil)
	})

	badRoundStore := newTestBadgerStore(t)
	err = badRoundStore.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
			return err
		}
		if err := writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 1, References: &common.RoundLink{}}); err != nil {
			return err
		}
		return writeTransaction(txn, snapshotVer)
	})
	require.Nil(err)
	require.Panics(func() {
		_ = badRoundStore.WriteSnapshot(snapshotWithTopoForTx(nodeID, 0, 1, 1, snapshotVer), nil)
	})

	missingTxStore := newTestBadgerStore(t)
	err = missingTxStore.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
			return err
		}
		return writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}})
	})
	require.Nil(err)
	require.Panics(func() {
		_ = missingTxStore.WriteSnapshot(snapshotWithTopoForTx(nodeID, 0, 1, 1, snapshotVer), nil)
	})
}

func TestRoundAssertionAndValidationEdges(t *testing.T) {
	require := require.New(t)

	nodeID := crypto.Blake3Hash([]byte("round-edge-node"))
	externalHash := crypto.Blake3Hash([]byte("round-edge-external-hash"))
	externalNode := crypto.Blake3Hash([]byte("round-edge-external-node"))
	selfFinal := crypto.Blake3Hash([]byte("round-edge-self-final"))
	refs := &common.RoundLink{Self: selfFinal, External: externalHash}

	writeRounds := func(store *BadgerStore, rounds ...*common.Round) {
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			for _, round := range rounds {
				if err := writeRound(txn, round.Hash, round); err != nil {
					return err
				}
			}
			return nil
		})
		require.Nil(err)
	}

	t.Run("StartNewRoundPanics", func(t *testing.T) {
		store := newTestBadgerStore(t)
		writeRounds(store, &common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}})
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})

		store = newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 5, References: &common.RoundLink{}},
			&common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})

		store = newTestBadgerStore(t)
		writeRounds(store, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}})
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})

		store = newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}},
			&common.Round{Hash: externalHash, NodeId: nodeID, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})

		store = newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}},
			&common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}},
			&common.Round{Hash: selfFinal, NodeId: nodeID, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})

		store = newTestBadgerStore(t)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: &common.RoundLink{}}); err != nil {
				return err
			}
			if err := writeRound(txn, externalHash, &common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}}); err != nil {
				return err
			}
			return writeLink(txn, nodeID, externalNode, 2)
		})
		require.Nil(err)
		require.Panics(func() {
			_ = store.StartNewRound(nodeID, 1, refs, 7)
		})
	})

	t.Run("UpdateEmptyHeadRoundPanics", func(t *testing.T) {
		store := newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 0, References: refs},
			&common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.UpdateEmptyHeadRound(nodeID, 1, refs)
		})

		store = newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 1, References: &common.RoundLink{Self: crypto.Blake3Hash([]byte("other-self")), External: externalHash}},
			&common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.UpdateEmptyHeadRound(nodeID, 1, refs)
		})

		store = newTestBadgerStore(t)
		writeRounds(store, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 1, References: refs})
		require.Panics(func() {
			_ = store.UpdateEmptyHeadRound(nodeID, 1, refs)
		})

		store = newTestBadgerStore(t)
		writeRounds(store,
			&common.Round{Hash: nodeID, NodeId: nodeID, Number: 1, References: refs},
			&common.Round{Hash: externalHash, NodeId: nodeID, Number: 0, References: &common.RoundLink{}},
		)
		require.Panics(func() {
			_ = store.UpdateEmptyHeadRound(nodeID, 1, refs)
		})

		store = newTestBadgerStore(t)
		existing := common.NewTransactionV5(common.XINAssetId)
		existing.Inputs = []*common.Input{{Genesis: []byte("round-existing")}}
		existing.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
		existingVer := existing.AsVersioned()
		existingSnap := snapshotWithTopoForTx(nodeID, 1, 1, 9, existingVer)
		err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
			if err := writeRound(txn, nodeID, &common.Round{Hash: nodeID, NodeId: nodeID, Number: 1, References: refs}); err != nil {
				return err
			}
			if err := writeRound(txn, externalHash, &common.Round{Hash: externalHash, NodeId: externalNode, Number: 0, References: &common.RoundLink{}}); err != nil {
				return err
			}
			return txn.Set(graphSnapshotKey(nodeID, 1, existingVer.PayloadHash()), existingSnap.VersionedMarshal())
		})
		require.Nil(err)
		require.Panics(func() {
			_ = store.UpdateEmptyHeadRound(nodeID, 1, refs)
		})
	})

	emptyStore := newTestBadgerStore(t)
	total, invalid, err := emptyStore.validateSnapshotEntriesForNode(nodeID, 10)
	require.Nil(err)
	require.Zero(total)
	require.Zero(invalid)

	validationStore := newTestBadgerStore(t)
	networkID := crypto.Blake3Hash([]byte("validation-edge-network"))
	signer := seededNodeAddress(150)
	payee := seededNodeAddress(151)
	validationNodeID := signer.Hash().ForNetwork(networkID)
	tx1 := common.NewTransactionV5(common.XINAssetId)
	tx1.Inputs = []*common.Input{{Genesis: []byte("validation-1")}}
	tx1.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	ver1 := tx1.AsVersioned()
	tx2 := common.NewTransactionV5(common.XINAssetId)
	tx2.Inputs = []*common.Input{{Genesis: []byte("validation-2")}}
	tx2.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	ver2 := tx2.AsVersioned()
	snap1 := snapshotWithTopoForTx(validationNodeID, 0, 1, 2, ver1)
	snap2 := snapshotWithTopoForTx(validationNodeID, 0, 2, 3, ver2)
	_, _, roundHash := computeRoundHash(validationNodeID, 0, []*common.SnapshotWithTopologicalOrder{snap1, snap2})

	err = validationStore.snapshotsDB.Update(func(txn *badger.Txn) error {
		err := writeAssetInfo(txn, common.XINAssetId, &common.Asset{
			Chain:    common.BitcoinAssetId,
			AssetKey: "btc",
		})
		if err != nil {
			return err
		}
		err = writeTransaction(txn, ver1)
		if err != nil {
			return err
		}
		err = writeSnapshot(txn, snap1, ver1)
		if err != nil {
			return err
		}
		err = writeTransaction(txn, ver2)
		if err != nil {
			return err
		}
		err = writeSnapshot(txn, snap2, ver2)
		if err != nil {
			return err
		}
		err = writeNodeAccept(txn, signer.PublicSpendKey, payee.PublicSpendKey, crypto.Blake3Hash([]byte("validation-node")), 1, true)
		if err != nil {
			return err
		}
		err = txn.Set(graphTransactionKey(ver1.PayloadHash()), ver2.Marshal())
		if err != nil {
			return err
		}
		snap2Hash := snap2.PayloadHash()
		err = txn.Set(graphFinalizationKey(ver1.PayloadHash()), snap2Hash[:])
		if err != nil {
			return err
		}
		err = writeRound(txn, roundHash, &common.Round{
			Hash:      roundHash,
			NodeId:    crypto.Blake3Hash([]byte("wrong-round-node")),
			Number:    9,
			Timestamp: snap2.Timestamp,
		})
		if err != nil {
			return err
		}
		return writeRound(txn, validationNodeID, &common.Round{
			Hash:      validationNodeID,
			NodeId:    validationNodeID,
			Number:    1,
			Timestamp: snap2.Timestamp,
		})
	})
	require.Nil(err)

	total, invalid, err = validationStore.validateSnapshotEntriesForNode(validationNodeID, 10)
	require.Nil(err)
	require.Equal(2, total)
	require.GreaterOrEqual(invalid, 2)

	total, invalid, err = validationStore.ValidateGraphEntries(networkID, 10)
	require.Nil(err)
	require.Equal(2, total)
	require.GreaterOrEqual(invalid, 2)

	badGapOne := &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       validationNodeID,
		Timestamp:    1,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("gap-one"))},
		Hash:         crypto.Blake3Hash([]byte("gap-hash-one")),
	}}
	badGapTwo := &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       validationNodeID,
		Timestamp:    uint64(config.SnapshotRoundGap) + 2,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("gap-two"))},
		Hash:         crypto.Blake3Hash([]byte("gap-hash-two")),
	}}
	require.Panics(func() {
		_, _, _ = computeRoundHash(validationNodeID, 0, []*common.SnapshotWithTopologicalOrder{badGapTwo, badGapOne})
	})
}

func TestGenesisLoadEdges(t *testing.T) {
	require := require.New(t)

	store := newTestBadgerStore(t)
	nodeID := crypto.Blake3Hash([]byte("genesis-node"))
	signer := seededAddress(170)
	payee := seededAddress(171)
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte("genesis-load")}}
	tx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
	tx.Extra = append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)
	ver := tx.AsVersioned()
	snap := snapshotWithTopoForTx(nodeID, 0, 0, 1, ver)

	loaded, err := store.CheckGenesisLoad([]*common.SnapshotWithTopologicalOrder{snap})
	require.Nil(err)
	require.False(loaded)

	err = store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{snap}, []*common.VersionedTransaction{ver})
	require.Nil(err)

	loaded, err = store.CheckGenesisLoad([]*common.SnapshotWithTopologicalOrder{snap})
	require.Nil(err)
	require.True(loaded)

	err = store.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{snap}, []*common.VersionedTransaction{ver})
	require.Nil(err)

	wrong := *snap
	wrong.Hash = crypto.Blake3Hash([]byte("wrong-genesis-hash"))
	loaded, err = store.CheckGenesisLoad([]*common.SnapshotWithTopologicalOrder{&wrong})
	require.True(loaded)
	require.ErrorContains(err, "malformed genesis snapshot")

	badTopoStore := newTestBadgerStore(t)
	badSnap := snapshotWithTopoForTx(nodeID, 0, 1, 1, ver)
	require.Panics(func() {
		_ = badTopoStore.LoadGenesis(nil, []*common.SnapshotWithTopologicalOrder{badSnap}, []*common.VersionedTransaction{ver})
	})
}

func TestGraphTopologyAndStoreEdges(t *testing.T) {
	require := require.New(t)

	badRoot := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(os.WriteFile(badRoot, []byte("x"), 0o644))
	_, err := NewBadgerStore(nil, badRoot)
	require.Error(err)

	emptyStore := newTestBadgerStore(t)
	missingHash := crypto.Blake3Hash([]byte("missing-snapshot"))
	snap, err := emptyStore.ReadSnapshot(missingHash)
	require.Nil(err)
	require.Nil(snap)
	require.Panics(func() {
		emptyStore.LastSnapshot()
	})

	nodeID := crypto.Blake3Hash([]byte("topology-edge-node"))
	signer := seededAddress(180)
	payee := seededAddress(181)
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte("topology-edge")}}
	tx.Outputs = []*common.Output{{Type: common.OutputTypeNodeAccept, Amount: common.NewInteger(1)}}
	tx.Extra = append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)
	ver := tx.AsVersioned()
	validSnap := snapshotWithTopoForTx(nodeID, 0, 1, 1, ver)
	snapHash := validSnap.PayloadHash()

	missingTopoTarget := newTestBadgerStore(t)
	err = missingTopoTarget.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphSnapTopologyKey(snapHash), graphTopologyKey(99))
	})
	require.Nil(err)
	_, err = missingTopoTarget.ReadSnapshot(snapHash)
	require.Error(err)

	missingSnapshotTarget := newTestBadgerStore(t)
	err = missingSnapshotTarget.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := txn.Set(graphSnapTopologyKey(snapHash), graphTopologyKey(1)); err != nil {
			return err
		}
		return txn.Set(graphTopologyKey(1), graphSnapshotKey(nodeID, 0, ver.PayloadHash()))
	})
	require.Nil(err)
	_, err = missingSnapshotTarget.ReadSnapshot(snapHash)
	require.Error(err)

	badSnapshotTarget := newTestBadgerStore(t)
	err = badSnapshotTarget.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := txn.Set(graphSnapTopologyKey(snapHash), graphTopologyKey(1)); err != nil {
			return err
		}
		if err := txn.Set(graphTopologyKey(1), graphSnapshotKey(nodeID, 0, ver.PayloadHash())); err != nil {
			return err
		}
		return txn.Set(graphSnapshotKey(nodeID, 0, ver.PayloadHash()), []byte{0})
	})
	require.Nil(err)
	_, err = badSnapshotTarget.ReadSnapshot(snapHash)
	require.Error(err)

	badSinceTarget := newTestBadgerStore(t)
	err = badSinceTarget.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := txn.Set(graphTopologyKey(1), graphSnapshotKey(nodeID, 0, ver.PayloadHash())); err != nil {
			return err
		}
		return txn.Set(graphSnapshotKey(nodeID, 0, ver.PayloadHash()), []byte{0})
	})
	require.Nil(err)
	_, err = badSinceTarget.ReadSnapshotsSinceTopology(1, 10)
	require.Error(err)

	badTxTarget := newTestBadgerStore(t)
	err = badTxTarget.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
			return err
		}
		if err := writeTransaction(txn, ver); err != nil {
			return err
		}
		if err := writeSnapshot(txn, validSnap, ver); err != nil {
			return err
		}
		return txn.Set(graphTransactionKey(ver.PayloadHash()), []byte{0})
	})
	require.Nil(err)
	_, _, err = badTxTarget.ReadSnapshotWithTransactionsSinceTopology(1, 10)
	require.Error(err)

	consensusMismatch := newTestBadgerStore(t)
	err = consensusMismatch.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
			return err
		}
		if err := writeTransaction(txn, ver); err != nil {
			return err
		}
		if err := writeSnapshot(txn, validSnap, ver); err != nil {
			return err
		}
		return txn.Set(graphConsensusSnapshotKey(validSnap.Timestamp+1, snapHash), []byte{})
	})
	require.Nil(err)
	require.Panics(func() {
		_, _ = consensusMismatch.ReadLastConsensusSnapshot()
	})

	consensusValue := newTestBadgerStore(t)
	err = consensusValue.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, common.XINAssetId, common.XINAsset); err != nil {
			return err
		}
		if err := writeTransaction(txn, ver); err != nil {
			return err
		}
		if err := writeSnapshot(txn, validSnap, ver); err != nil {
			return err
		}
		txHash := ver.PayloadHash()
		return txn.Set(graphConsensusSnapshotKey(validSnap.Timestamp, snapHash), txHash[:])
	})
	require.Nil(err)
	require.Panics(func() {
		_, _ = consensusValue.ReadLastConsensusSnapshot()
	})

	writeConsensusPanic := newTestBadgerStore(t)
	scriptTx := common.NewTransactionV5(common.XINAssetId)
	scriptTx.Inputs = []*common.Input{{Genesis: []byte("script-consensus")}}
	scriptTx.Outputs = []*common.Output{{Type: common.OutputTypeScript, Amount: common.NewInteger(1)}}
	scriptVer := scriptTx.AsVersioned()
	scriptSnap := snapshotWithTopoForTx(nodeID, 0, 1, 1, scriptVer)
	err = writeConsensusPanic.snapshotsDB.Update(func(txn *badger.Txn) error {
		require.Panics(func() {
			_ = writeConsensusSnapshot(txn, scriptSnap.Snapshot, scriptVer, nil)
		})
		return nil
	})
	require.Nil(err)
}

func newTestBadgerStore(t *testing.T) *BadgerStore {
	t.Helper()

	custom, err := config.Initialize("../config/config.example.toml")
	require.NoError(t, err)

	store, err := NewBadgerStore(custom, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})
	return store
}

func snapshotWithTopoForTx(nodeID crypto.Hash, round, topo, timestamp uint64, ver *common.VersionedTransaction) *common.SnapshotWithTopologicalOrder {
	snap := &common.SnapshotWithTopologicalOrder{
		Snapshot: &common.Snapshot{
			Version:      common.SnapshotVersionCommonEncoding,
			NodeId:       nodeID,
			RoundNumber:  round,
			Timestamp:    timestamp,
			Transactions: []crypto.Hash{ver.PayloadHash()},
		},
		TopologicalOrder: topo,
	}
	snap.Hash = snap.PayloadHash()
	return snap
}

func seededPublicKey(seed byte) crypto.Key {
	src := make([]byte, 64)
	for i := range src {
		src[i] = seed
	}
	return crypto.NewKeyFromSeed(src).Public()
}

func seededAddress(seed byte) common.Address {
	src := make([]byte, 64)
	for i := range src {
		src[i] = seed
	}
	return common.NewAddressFromSeed(src)
}

func nodeOpTransaction(outputType uint8) *common.VersionedTransaction {
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Outputs = []*common.Output{{Type: outputType, Amount: common.NewInteger(1)}}
	return tx.AsVersioned()
}

func seededNodeAddress(seed byte) common.Address {
	src := make([]byte, 64)
	for i := range src {
		src[i] = seed
	}
	spend := crypto.NewKeyFromSeed(src)
	privateView := spend.Public().DeterministicHashDerive()
	return common.Address{
		PrivateSpendKey: spend,
		PrivateViewKey:  privateView,
		PublicSpendKey:  spend.Public(),
		PublicViewKey:   privateView.Public(),
	}
}

func buildCustodianUpdateTransaction(seed byte, networkID crypto.Hash) (*common.VersionedTransaction, []byte) {
	current := seededAddress(seed)
	nodes := make([][]byte, 0, 7)
	for i := 0; i < 7; i += 1 {
		custodian := seededAddress(seed + byte(i*3+1))
		payee := seededAddress(seed + byte(i*3+2))
		signer := seededAddress(seed + byte(i*3+3))
		extra := common.EncodeCustodianNode(
			&custodian,
			&payee,
			&signer.PrivateSpendKey,
			&payee.PrivateSpendKey,
			&custodian.PrivateSpendKey,
			networkID,
		)
		nodes = append(nodes, extra)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i][1:33], nodes[j][1:33]) < 0
	})

	extra := append(current.PublicSpendKey[:], current.PublicViewKey[:]...)
	for _, node := range nodes {
		extra = append(extra, node...)
	}
	sig := current.PrivateSpendKey.Sign(crypto.Blake3Hash(extra))
	extra = append(extra, sig[:]...)

	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte("genesis")}}
	tx.Outputs = []*common.Output{{Type: common.OutputTypeCustodianUpdateNodes, Amount: common.NewInteger(1)}}
	tx.Extra = extra
	return tx.AsVersioned(), extra
}
