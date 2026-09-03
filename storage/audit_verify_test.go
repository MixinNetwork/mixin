package storage

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func auditStore(t *testing.T) *BadgerStore {
	custom, err := config.Initialize("../config/config.example.toml")
	require.NoError(t, err)
	store, err := NewBadgerStore(custom, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { util.CloseOrPanic(store) })
	return store
}

// Q0: does badger Txn.Get see its own pending writes? The comment in
// badger_transaction.go claims it does not. This determines whether
// same-snapshot double withdrawal claims panic or silently succeed.
func TestAuditBadgerGetReadsPendingWrites(t *testing.T) {
	store := auditStore(t)
	key := []byte("audit-pending-key")
	err := store.snapshotsDB.Update(func(txn *badger.Txn) error {
		require.NoError(t, txn.Set(key, []byte("v1")))
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			t.Log("RESULT: Get does NOT see pending writes")
			return nil
		}
		require.NoError(t, err)
		val, err := item.ValueCopy(nil)
		require.NoError(t, err)
		t.Logf("RESULT: Get SEES pending writes: %s", string(val))
		return nil
	})
	require.NoError(t, err)
}

func auditSnap(store *BadgerStore, nodeId crypto.Hash, round uint64, ts uint64) *common.SnapshotWithTopologicalOrder {
	s := &common.Snapshot{
		Version:     common.SnapshotVersionCommonEncoding,
		NodeId:      nodeId,
		RoundNumber: round,
		Timestamp:   ts,
		References:  &common.RoundLink{},
	}
	return &common.SnapshotWithTopologicalOrder{Snapshot: s, TopologicalOrder: round}
}

// auditFinalize runs the same writeSnapshot WriteSnapshot uses, minus the
// debug-only round asserts that require full kernel round setup.
func auditFinalize(store *BadgerStore, snap *common.SnapshotWithTopologicalOrder) error {
	return store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeSnapshot(txn, snap)
	})
}

func auditPutAssetTotal(t *testing.T, store *BadgerStore, total string) {
	t.Helper()
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphAssetTotalKey(common.XINAssetId), []byte(total))
	}))
}

// Q1: double-spend of a UTXO by two transactions where the first is FINALIZED.
// The second must be rejected even on the fork=true finalization path.
func TestAuditDoubleSpendAfterFinalization(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)

	funding, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)
	_ = funding

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)
	require.NotEqual(spend1.PayloadHash(), spend2.PayloadHash())

	// persist + lock spend1
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// finalize spend1 via WriteSnapshot
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := auditSnap(store, nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{spend1.PayloadHash()}
	require.NoError(auditFinalize(store, snap))

	// spend1 must be finalized now
	_, fin, err := store.ReadTransaction(spend1.PayloadHash())
	require.NoError(err)
	require.NotEmpty(fin)

	// attempt to lock+persist spend2 on the fork=true finalization path
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true)
	t.Logf("RESULT: conflicting finalized spend lock attempt err=%v", err)
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

// Q1b: same-batch conflicting spends must be rejected.
func TestAuditDoubleSpendSameBatch(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, false)
	t.Logf("RESULT: same-batch double spend err=%v", err)
	require.Error(err)

	// neither should be persisted (atomic rollback)
	batchIntegrityRequireTransactionMissing(t, store, spend1)
	batchIntegrityRequireTransactionMissing(t, store, spend2)
}

// Q4: two withdrawal-claim transactions for the same withdrawal submit in the
// SAME snapshot/badger txn. Panic (crash) or silent double-claim (theft)?
func TestAuditDoubleWithdrawalClaimSameSnapshot(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	batchIntegrityPutAsset(t, store)
	auditPutAssetTotal(t, store, "100")

	// withdrawal submit tx (funded from a genesis-like utxo)
	funding, utxo := batchIntegrityFunding(10)
	_ = funding
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
	snap0 := auditSnap(store, nodeId, 0, 100)
	snap0.Transactions = []crypto.Hash{submitVer.PayloadHash()}
	require.NoError(auditFinalize(store, snap0))

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

	// lock+persist both claims (batchClaims has no withdrawal claim tracking)
	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{claim1, claim2}, false)
	t.Logf("RESULT: batch lock of two claims err=%v", err)
	if err != nil {
		return // rejected at lock time; safe
	}

	// finalize both in one WriteSnapshot (one badger txn)
	snap1 := auditSnap(store, nodeId, 1, 200)
	snap1.Transactions = []crypto.Hash{claim1.PayloadHash(), claim2.PayloadHash()}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("RESULT: WriteSnapshot with two claims PANICS: %v", r)
			}
		}()
		err := auditFinalize(store, snap1)
		t.Logf("RESULT: WriteSnapshot with two claims err=%v", err)
		require.NoError(err)
	}()

	// check claim record and finalization state
	claimRec, _, err := store.ReadWithdrawalClaim(submitVer.PayloadHash())
	require.NoError(err)
	t.Logf("RESULT: claim record=%v", claimRec != nil)
	_, fin1, _ := store.ReadTransaction(claim1.PayloadHash())
	_, fin2, _ := store.ReadTransaction(claim2.PayloadHash())
	t.Logf("RESULT: claim1 finalized=%q claim2 finalized=%q", fin1, fin2)
}

// Q5: deposit replay after finalization.
func TestAuditDepositReplayAfterFinalization(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	batchIntegrityPutAsset(t, store)

	dep1 := batchIntegrityDeposit("external-tx-1", 1, "0xa974c709cfb4566686553a20790685a47aceaa33")
	// same UniqueKey, different tx payload (different output seed => different hash)
	dep2 := batchIntegrityDeposit("external-tx-1", 2, "0xa974c709cfb4566686553a20790685a47aceaa33")
	require.Equal(dep1.Inputs[0].Deposit.UniqueKey(), dep2.Inputs[0].Deposit.UniqueKey())
	require.NotEqual(dep1.PayloadHash(), dep2.PayloadHash())

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{dep1}, false))
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := auditSnap(store, nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{dep1.PayloadHash()}
	require.NoError(auditFinalize(store, snap))

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{dep2}, true)
	t.Logf("RESULT: deposit replay after finalization err=%v", err)
	require.Error(err)

	// same batch replay
	dep3 := batchIntegrityDeposit("external-tx-2", 3, "0xa974c709cfb4566686553a20790685a47aceaa33")
	dep4 := batchIntegrityDeposit("external-tx-2", 4, "0xa974c709cfb4566686553a20790685a47aceaa33")
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{dep3, dep4}, false)
	t.Logf("RESULT: same-batch deposit replay err=%v", err)
	require.Error(err)
}

// Q6: mint double distribution.
func TestAuditMintDoubleDistribution(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	batchIntegrityPutAsset(t, store)

	mint1 := batchIntegrityMint(7, 1)
	mint2 := batchIntegrityMint(7, 2)
	require.NotEqual(mint1.PayloadHash(), mint2.PayloadHash())

	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{mint1}, false))
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := auditSnap(store, nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{mint1.PayloadHash()}
	require.NoError(auditFinalize(store, snap))

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{mint2}, true)
	t.Logf("RESULT: mint double distribution after finalization err=%v", err)
	require.Error(err)

	mint3 := batchIntegrityMint(8, 3)
	mint4 := batchIntegrityMint(8, 4)
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{mint3, mint4}, false)
	t.Logf("RESULT: same-batch mint double distribution err=%v", err)
	require.Error(err)
}

// Q3: ghost key reuse in two transactions.
func TestAuditGhostKeyReuse(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
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
	t.Logf("RESULT: ghost reuse fork=true err=%v", err)
	require.Error(err)

	// same batch
	err = store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, false)
	t.Logf("RESULT: ghost reuse same batch err=%v", err)
	require.Error(err)
}

// Q1d: same-batch conflicting spends on the fork=true finalization path.
func TestAuditDoubleSpendSameBatchFork(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1, spend2}, true)
	t.Logf("RESULT: same-batch double spend fork=true err=%v", err)
	require.Error(err)
	batchIntegrityRequireTransactionMissing(t, store, spend1)
	batchIntegrityRequireTransactionMissing(t, store, spend2)
}

// Q1e: cross-snapshot scenario - spend1 persisted earlier (found-path skip),
// then a finalized snapshot contains both spend1 and spend2. Verify the
// outcome is a panic with atomic discard, never a double finalization.
func TestAuditCrossSnapshotConflictFoundSkip(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
	_, utxo := batchIntegrityFunding(1)
	batchIntegrityPutAsset(t, store)
	batchIntegrityPutUTXO(t, store, utxo)

	spend1 := batchIntegritySpend(utxo, 2)
	spend2 := batchIntegritySpend(utxo, 3)

	// spend1 locked+persisted earlier, never finalized
	require.NoError(store.LockAndPersistTransactions([]*common.VersionedTransaction{spend1}, false))

	// finalization path for snapshot [spend1, spend2]:
	// spend1 is found in store (skipped), spend2 is locked+persisted with
	// fork=true, which prunes spend1's body.
	err := store.LockAndPersistTransactions([]*common.VersionedTransaction{spend2}, true)
	require.NoError(err)
	batchIntegrityRequireTransactionMissing(t, store, spend1)

	// WriteSnapshot over [spend2, spend1] must not double-finalize
	nodeId := crypto.Blake3Hash([]byte("node-a"))
	snap := auditSnap(store, nodeId, 1, 100)
	snap.Transactions = []crypto.Hash{spend2.PayloadHash(), spend1.PayloadHash()}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("RESULT: WriteSnapshot panics (atomic discard): %v", r)
			}
		}()
		err := auditFinalize(store, snap)
		t.Logf("RESULT: WriteSnapshot err=%v", err)
	}()

	// verify neither is finalized (atomic discard) and no double outputs
	_, fin1, _ := store.ReadTransaction(spend1.PayloadHash())
	_, fin2, _ := store.ReadTransaction(spend2.PayloadHash())
	t.Logf("RESULT: spend1 finalized=%q spend2 finalized=%q", fin1, fin2)
	require.Empty(fin1)
	out, err := store.ReadUTXOLock(spend2.PayloadHash(), 0)
	require.NoError(err)
	if fin2 == "" {
		require.Nil(out) // spend2 outputs must not exist either
		t.Log("RESULT: atomic discard confirmed, no double spend")
	} else {
		require.NotNil(out)
		t.Log("RESULT: only spend2 finalized, single spend")
	}
}

// Q1c: fork=true path against an UNFINALIZED lock holder prunes it; check that
// ghost keys of the pruned tx remain locked (burn) and that the winner can finalize.
func TestAuditForkPruneUnfinalized(t *testing.T) {
	require := require.New(t)
	store := auditStore(t)
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
	snap := auditSnap(store, nodeId, 0, 100)
	snap.Transactions = []crypto.Hash{spend2.PayloadHash()}
	require.NoError(auditFinalize(store, snap))

	// spend1 ghost keys remain locked by spend1 (burned)
	for _, k := range spend1.Outputs[0].Keys {
		by, err := store.ReadGhostKeyLock(*k)
		require.NoError(err)
		t.Logf("RESULT: pruned tx ghost key locked by=%s (spend1=%s)", by, spend1.PayloadHash())
	}
}
