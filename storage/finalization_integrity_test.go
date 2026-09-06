package storage

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestBadgerTransactionReadsPendingWrites(t *testing.T) {
	store := newTestBadgerStore(t)
	key := []byte("pending-write-key")
	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		require.NoError(t, txn.Set(key, []byte("first")))
		item, err := txn.Get(key)
		require.NoError(t, err)
		value, err := item.ValueCopy(nil)
		require.NoError(t, err)
		require.Equal(t, "first", string(value))

		require.NoError(t, txn.Set(key, []byte("second")))
		item, err = txn.Get(key)
		require.NoError(t, err)
		value, err = item.ValueCopy(nil)
		require.NoError(t, err)
		require.Equal(t, "second", string(value))

		require.NoError(t, txn.Delete(key))
		_, err = txn.Get(key)
		require.ErrorIs(t, err, badger.ErrKeyNotFound)
		return nil
	})
	require.NoError(t, err)
	batchIntegrityRequireMissing(t, store, key)
}

func finalizationTestSnapshot(nodeId crypto.Hash, round uint64, ts uint64) *common.SnapshotWithTopologicalOrder {
	s := &common.Snapshot{
		Version:     common.SnapshotVersionCommonEncoding,
		NodeId:      nodeId,
		RoundNumber: round,
		Timestamp:   ts,
	}
	if round > 0 {
		s.References = &common.RoundLink{}
	}
	return &common.SnapshotWithTopologicalOrder{Snapshot: s, TopologicalOrder: round}
}

// finalizeTestSnapshot exercises storage finalization with canonical wire data.
// It omits the public wrapper's round assertions, which require kernel setup.
func finalizeTestSnapshot(store *BadgerStore, snap *common.SnapshotWithTopologicalOrder) error {
	decoded, err := common.UnmarshalVersionedSnapshot(snap.VersionedMarshal())
	if err != nil {
		return err
	}
	return store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeSnapshot(txn, decoded)
	})
}

func putFinalizationTestAssetTotal(t *testing.T, store *BadgerStore, total string) {
	t.Helper()
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphAssetTotalKey(common.XINAssetId), []byte(total))
	}))
}

func TestFinalizedSpendRejectsConflictingFork(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)

	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)
	require.NotEqual(spend1.PayloadHash(), spend2.PayloadHash())

	// persist + lock spend1
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// Finalize spend1 so its input lock cannot be replaced.
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := finalizationTestSnapshot(nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{spend1.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, snap))

	// spend1 must be finalized now
	_, fin, err := store.ReadTransaction(spend1.PayloadHash())
	require.NoError(err)
	require.NotEmpty(fin)

	// attempt to lock+persist spend2 on the fork=true finalization path
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true)
	require.Error(err)

	// spend2 must not be persisted
	persisted, _, err := store.ReadTransaction(spend2.PayloadHash())
	require.NoError(err)
	require.Nil(persisted)

	// utxo lock must still belong to spend1
	out, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
	require.NoError(err)
	require.Equal(spend1.PayloadHash(), out.LockHash)
}

func TestTransactionBatchRejectsDoubleSpend(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, false)
	require.Error(err)

	// neither should be persisted (atomic rollback)
	batchIntegrityRequireTransactionMissing(t, store, spend1)
	batchIntegrityRequireTransactionMissing(t, store, spend2)
}

func TestSnapshotRejectsDuplicateWithdrawalClaimsAtomically(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	batchIntegrityPutAsset(t, store)
	putFinalizationTestAssetTotal(t, store, "100")

	// withdrawal submit tx (funded from a genesis-like utxo)
	_, utxo := batchIntegrityFunding(10)
	batchIntegrityPutUTXO(t, store, utxo)

	submit := common.NewTransactionV5(common.XINAssetId)
	submit.AddInput(utxo.Hash, utxo.Index)
	submit.Outputs = append(submit.Outputs, &common.Output{
		Type:       common.OutputTypeWithdrawalSubmit,
		Amount:     common.NewInteger(1),
		Withdrawal: &common.WithdrawalData{Address: "bc1qwhatever"},
	})
	submitVer := submit.AsVersioned()

	// lock+persist+finalize the submit
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{submitVer}, false))
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap0 := finalizationTestSnapshot(nodeId, 0, 100)
	snap0.Transactions = []crypto.Hash{submitVer.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, snap0))

	// two claim txs, each funded by their own utxo, both referencing submit
	mkClaim := func(seed byte) *common.VersionedTransaction {
		_, fu := batchIntegrityFunding(seed)
		batchIntegrityPutUTXO(t, store, fu)
		c := common.NewTransactionV5(common.XINAssetId)
		c.AddInput(fu.Hash, fu.Index)
		c.Outputs = append(c.Outputs, &common.Output{
			Type:   common.OutputTypeWithdrawalClaim,
			Amount: common.NewInteger(1),
		})
		c.References = []crypto.Hash{submitVer.PayloadHash()}
		c.Extra = make([]byte, 96)
		return c.AsVersioned()
	}
	claim1 := mkClaim(11)
	claim2 := mkClaim(12)
	require.NotEqual(claim1.PayloadHash(), claim2.PayloadHash())

	// Claim uniqueness is enforced during finalization, after both candidates persist.
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{claim1, claim2}, false))
	_, balanceBefore, err := store.ReadAssetWithBalance(common.XINAssetId)
	require.NoError(err)

	snap1 := finalizationTestSnapshot(nodeId, 1, 200)
	snap1.Transactions = []crypto.Hash{claim1.PayloadHash(), claim2.PayloadHash()}
	snap1.Hash = snap1.PayloadHash()
	require.PanicsWithError("already claimed by "+snap1.Transactions[0].String(), func() {
		require.NoError(finalizeTestSnapshot(store, snap1))
	})

	claimRec, claimFinalization, err := store.ReadWithdrawalClaim(submitVer.PayloadHash())
	require.NoError(err)
	require.Nil(claimRec)
	require.Empty(claimFinalization)
	requireSnapshotNotFinalized(t, store, snap1)
	_, balanceAfter, err := store.ReadAssetWithBalance(common.XINAssetId)
	require.NoError(err)
	require.Equal(balanceBefore, balanceAfter)

	// A single claim remains finalizable after the failed batch.
	single := finalizationTestSnapshot(nodeId, 1, 201)
	single.Transactions = []crypto.Hash{claim1.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, single))
	claimRec, claimFinalization, err = store.ReadWithdrawalClaim(submitVer.PayloadHash())
	require.NoError(err)
	require.NotNil(claimRec)
	require.Equal(claim1.PayloadHash(), claimRec.PayloadHash())
	require.Equal(single.PayloadHash().String(), claimFinalization)

	duplicate := finalizationTestSnapshot(nodeId, 2, 202)
	duplicate.Transactions = []crypto.Hash{claim2.PayloadHash()}
	require.PanicsWithError("already claimed by "+claim1.PayloadHash().String(), func() {
		require.NoError(finalizeTestSnapshot(store, duplicate))
	})
	requireSnapshotNotFinalized(t, store, duplicate)
}

func TestDepositLocksRejectReplay(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	batchIntegrityPutAsset(t, store)

	dep1 := batchIntegrityDeposit("external-tx-1", 1, "0xa974c709cfb4566686553a20790685a47aceaa33")
	// same UniqueKey, different tx payload (different output seed => different hash)
	dep2 := batchIntegrityDeposit("external-tx-1", 2, "0xa974c709cfb4566686553a20790685a47aceaa33")
	require.Equal(dep1.Inputs[0].Deposit.UniqueKey(), dep2.Inputs[0].Deposit.UniqueKey())
	require.NotEqual(dep1.PayloadHash(), dep2.PayloadHash())

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{dep1}, false))
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := finalizationTestSnapshot(nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{dep1.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, snap))

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{dep2}, true)
	require.Error(err)

	// same batch replay
	dep3 := batchIntegrityDeposit("external-tx-2", 3, "0xa974c709cfb4566686553a20790685a47aceaa33")
	dep4 := batchIntegrityDeposit("external-tx-2", 4, "0xa974c709cfb4566686553a20790685a47aceaa33")
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{dep3, dep4}, false)
	require.Error(err)
}

func TestMintLocksRejectDuplicateDistribution(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	batchIntegrityPutAsset(t, store)

	mint1 := batchIntegrityMint(7, 1)
	mint2 := batchIntegrityMint(7, 2)
	require.NotEqual(mint1.PayloadHash(), mint2.PayloadHash())

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{mint1}, false))
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := finalizationTestSnapshot(nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{mint1.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, snap))

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{mint2}, true)
	require.Error(err)

	mint3 := batchIntegrityMint(8, 3)
	mint4 := batchIntegrityMint(8, 4)
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{mint3, mint4}, false)
	require.Error(err)
}

func TestGhostLocksRejectReuseAcrossTransactions(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	batchIntegrityPutAsset(t, store)

	_, utxo1 := batchIntegrityFunding(1)
	_, utxo2 := batchIntegrityFunding(2)
	batchIntegrityPutUTXO(t, store, utxo1)
	batchIntegrityPutUTXO(t, store, utxo2)

	// two spends with the SAME output keys (same seed => same ghost keys)
	spend1 := batchIntegritySpend(utxo1, 5)
	spend2 := batchIntegritySpend(utxo2, 5)
	require.Equal(spend1.Outputs[0].Keys[0].String(), spend2.Outputs[0].Keys[0].String())

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// even on fork=true path, ghost reuse must fail
	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true)
	require.Error(err)

	// same batch
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, false)
	require.Error(err)
}

func TestTransactionBatchRejectsDoubleSpendWithFork(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, true)
	require.Error(err)
	batchIntegrityRequireTransactionMissing(t, store, spend1)
	batchIntegrityRequireTransactionMissing(t, store, spend2)
}

func TestSnapshotWithPrunedTransactionRollsBack(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)
	firstHash, secondHash := spend1.PayloadHash(), spend2.PayloadHash()
	if bytes.Compare(firstHash[:], secondHash[:]) < 0 {
		spend1, spend2 = spend2, spend1
	}

	// spend1 locked+persisted earlier, never finalized
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// finalization path for snapshot [spend1, spend2]:
	// spend1 is found in store (skipped), spend2 is locked+persisted with
	// fork=true, which prunes spend1's body.
	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true)
	require.NoError(err)
	batchIntegrityRequireTransactionMissing(t, store, spend1)

	// The pruned envelope sorts after the valid candidate, so rejection must
	// roll back finalization writes already staged for that candidate.
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := finalizationTestSnapshot(nodeId, 1, 100)
	snap.Transactions = []crypto.Hash{spend2.PayloadHash(), spend1.PayloadHash()}
	require.Panics(func() {
		require.NoError(finalizeTestSnapshot(store, snap))
	})
	requireSnapshotNotFinalized(t, store, snap)

	// The replacement candidate and its input lock survive the rejected snapshot.
	batchIntegrityRequireTransaction(t, store, spend2)
	out, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
	require.NoError(err)
	require.NotNil(out)
	require.Equal(spend2.PayloadHash(), out.LockHash)

	single := finalizationTestSnapshot(nodeId, 1, 101)
	single.Transactions = []crypto.Hash{spend2.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, single))
	batchIntegrityRequireFinalizedTransaction(t, store, spend2)
}

func TestForkPrunesUnfinalizedTransactionAndPreservesGhostLocks(t *testing.T) {
	require := require.New(t)
	store := newTestBadgerStore(t)
	batchIntegrityPutAsset(t, store)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// fork path: spend2 wins over unfinalized spend1
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true))

	// spend1 body pruned
	batchIntegrityRequireTransactionMissing(t, store, spend1)

	// spend2 finalizes fine
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := finalizationTestSnapshot(nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{spend2.PayloadHash()}
	require.NoError(finalizeTestSnapshot(store, snap))

	// The pruned candidate retains ownership of its ghost keys.
	for _, k := range spend1.Outputs[0].Keys {
		by, err := store.ReadGhostKeyLock(*k)
		require.NoError(err)
		require.NotNil(by)
		require.Equal(spend1.PayloadHash(), *by)
	}
}

func requireSnapshotNotFinalized(t *testing.T, store *BadgerStore, snap *common.SnapshotWithTopologicalOrder) {
	t.Helper()
	for _, hash := range snap.Transactions {
		_, finalized, err := store.ReadTransaction(hash)
		require.NoError(t, err)
		require.Empty(t, finalized)
		output, err := store.ReadUTXOLock(hash, 0)
		require.NoError(t, err)
		require.Nil(t, output)
		batchIntegrityRequireMissing(t, store, graphUniqueKey(snap.NodeId, hash))
	}
	batchIntegrityRequireMissing(t, store, graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.PayloadHash()))
	batchIntegrityRequireMissing(t, store, graphSnapTopologyKey(snap.PayloadHash()))
	batchIntegrityRequireMissing(t, store, graphTopologyKey(snap.TopologicalOrder))
}
