package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestStorageHelpersPropagateDiscardedTransactionErrors(t *testing.T) {
	store := newTestBadgerStore(t)
	id := crypto.Blake3Hash([]byte("discarded transaction"))
	asset := &common.Asset{Chain: common.BitcoinAssetId, AssetKey: "discarded"}
	mint := &common.MintData{Group: "UNIVERSAL", Batch: 1, Amount: common.NewInteger(1)}
	deposit := &common.DepositData{
		Chain:       common.BitcoinAssetId,
		AssetKey:    "discarded",
		Transaction: "discarded",
		Amount:      common.NewInteger(1),
	}
	ver := common.NewTransactionV5(id).AsVersioned()
	snap := &common.SnapshotWithTopologicalOrder{Snapshot: &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       id,
		Transactions: []crypto.Hash{id},
		Hash:         id,
	}}
	utxo := &common.UTXOWithLock{UTXO: common.UTXO{
		Input: common.Input{Hash: id},
		Output: common.Output{
			Type:   common.OutputTypeScript,
			Amount: common.NewInteger(1),
		},
	}}

	discarded := func(db *badger.DB, update bool) *badger.Txn {
		t.Helper()
		txn := db.NewTransaction(update)
		txn.Discard()
		return txn
	}
	requireDiscarded := func(err error) {
		t.Helper()
		require.ErrorIs(t, err, badger.ErrDiscardedTxn)
	}

	_, err := readTotalInAsset(discarded(store.snapshotsDB, false), id)
	requireDiscarded(err)
	_, err = readAssetInfo(discarded(store.snapshotsDB, false), id)
	requireDiscarded(err)
	requireDiscarded(verifyAssetInfo(discarded(store.snapshotsDB, false), id, asset))
	requireDiscarded(writeAssetInfo(discarded(store.snapshotsDB, true), id, asset))
	requireDiscarded(writeTotalInAsset(discarded(store.snapshotsDB, true), ver))

	_, err = store.cacheReadTransaction(discarded(store.cacheDB, false), id)
	requireDiscarded(err)
	_, err = readDepositInput(discarded(store.snapshotsDB, false), deposit)
	requireDiscarded(err)
	requireDiscarded(writeDepositLock(discarded(store.snapshotsDB, true), deposit, id))
	_, err = readMintInput(discarded(store.snapshotsDB, false), mint)
	requireDiscarded(err)
	requireDiscarded(writeMintDistribution(discarded(store.snapshotsDB, true), mint, id))

	_, err = readLink(discarded(store.snapshotsDB, false), id, id)
	requireDiscarded(err)
	requireDiscarded(writeLink(discarded(store.snapshotsDB, true), id, id, 1))
	_, err = readRound(discarded(store.snapshotsDB, false), id)
	requireDiscarded(err)
	requireDiscarded(writeRound(discarded(store.snapshotsDB, true), id, &common.Round{Hash: id}))
	requireDiscarded(startNewRound(discarded(store.snapshotsDB, true), id, 0, &common.RoundLink{}, 0))

	_, _, err = readRoundSpaceCheckpoint(discarded(store.snapshotsDB, false), id)
	requireDiscarded(err)
	_, err = readSnapshotWithTopo(discarded(store.snapshotsDB, false), id)
	requireDiscarded(err)
	requireDiscarded(pruneTransaction(discarded(store.snapshotsDB, true), id))
	requireDiscarded(writeTransaction(discarded(store.snapshotsDB, true), ver))
	requireDiscarded(finalizeTransaction(discarded(store.snapshotsDB, true), ver, snap))
	requireDiscarded(writeUTXO(discarded(store.snapshotsDB, true), utxo, ver, 1, false))

	_, err = store.readUTXOLock(discarded(store.snapshotsDB, false), id, 0)
	requireDiscarded(err)
	requireDiscarded(lockUTXO(discarded(store.snapshotsDB, true), id, 0, id, false))
	ghost := seededPublicKey(211)
	requireDiscarded(lockGhostKey(discarded(store.snapshotsDB, true), &ghost, id, false))

	workKey := graphWorkOffsetKey(id)
	_, _, err = graphReadWorkOffset(discarded(store.snapshotsDB, false), workKey)
	requireDiscarded(err)
	requireDiscarded(graphWriteWorkOffset(discarded(store.snapshotsDB, true), workKey, 1, nil))
	_, err = graphReadUint64(discarded(store.snapshotsDB, false), workKey)
	requireDiscarded(err)
	requireDiscarded(graphWriteUint64(discarded(store.snapshotsDB, true), workKey, 1))
	requireDiscarded(writeSnapshotWork(discarded(store.snapshotsDB, true), snap, nil))
}
