package storage

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func batchIntegrityFunding(seed byte) (*common.VersionedTransaction, *common.UTXOWithLock) {
	address := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.Inputs = []*common.Input{{Genesis: []byte{seed}}}
	tx.AddScriptOutput(
		[]*common.Address{&address},
		common.NewThresholdScript(1),
		common.NewInteger(1),
		bytes.Repeat([]byte{seed + 1}, 64),
	)
	ver := tx.AsVersioned()
	return ver, ver.UnspentOutputs()[0]
}

func batchIntegritySpend(utxo *common.UTXOWithLock, seed byte) *common.VersionedTransaction {
	address := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddInput(utxo.Hash, utxo.Index)
	tx.AddScriptOutput(
		[]*common.Address{&address},
		common.NewThresholdScript(1),
		utxo.Amount,
		bytes.Repeat([]byte{seed + 1}, 64),
	)
	return tx.AsVersioned()
}

func batchIntegrityDeposit(nonce string, seed byte, assetKey string) *common.VersionedTransaction {
	address := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddDepositInput(&common.DepositData{
		Chain:       common.XINAsset.Chain,
		AssetKey:    assetKey,
		Transaction: nonce,
		Amount:      common.NewInteger(1),
	})
	tx.AddScriptOutput(
		[]*common.Address{&address},
		common.NewThresholdScript(1),
		common.NewInteger(1),
		bytes.Repeat([]byte{seed + 1}, 64),
	)
	return tx.AsVersioned()
}

func batchIntegrityMint(batch uint64, seed byte) *common.VersionedTransaction {
	address := common.NewAddressFromSeed(bytes.Repeat([]byte{seed}, 64))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddUniversalMintInput(batch, common.NewInteger(1))
	tx.AddScriptOutput(
		[]*common.Address{&address},
		common.NewThresholdScript(1),
		common.NewInteger(1),
		bytes.Repeat([]byte{seed + 1}, 64),
	)
	return tx.AsVersioned()
}

func batchIntegrityPutUTXO(t *testing.T, store *BadgerStore, utxo *common.UTXOWithLock) {
	t.Helper()
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Set(graphUtxoKey(utxo.Hash, utxo.Index), utxo.Marshal())
	}))
}

func batchIntegrityPutAsset(t *testing.T, store *BadgerStore) {
	t.Helper()
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeAssetInfo(txn, common.XINAssetId, common.XINAsset)
	}))
}

func batchIntegrityRequireMissing(t *testing.T, store *BadgerStore, key []byte) {
	t.Helper()
	txn := store.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	_, err := txn.Get(key)
	require.ErrorIs(t, err, badger.ErrKeyNotFound)
}

func batchIntegrityRequireTransactionMissing(t *testing.T, store *BadgerStore, tx *common.VersionedTransaction) {
	t.Helper()
	persisted, finalizedBy, err := store.ReadTransaction(tx.PayloadHash())
	require.NoError(t, err)
	require.Nil(t, persisted)
	require.Empty(t, finalizedBy)
}

func batchIntegrityRequireTransaction(t *testing.T, store *BadgerStore, expected *common.VersionedTransaction) {
	t.Helper()
	persisted, _, err := store.ReadTransaction(expected.PayloadHash())
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, expected.Marshal(), persisted.Marshal())
}

func batchIntegrityRequireFinalizedTransaction(t *testing.T, store *BadgerStore, expected *common.VersionedTransaction) {
	t.Helper()
	persisted, finalizedBy, err := store.ReadTransaction(expected.PayloadHash())
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, expected.Marshal(), persisted.Marshal())
	require.NotEmpty(t, finalizedBy)
}

func batchIntegrityRequireGhostMissing(t *testing.T, store *BadgerStore, tx *common.VersionedTransaction) {
	t.Helper()
	for _, output := range tx.Outputs {
		for _, key := range output.Keys {
			batchIntegrityRequireMissing(t, store, graphGhostKey(*key))
		}
	}
}

func batchIntegrityRequireNoNewRecords(t *testing.T, store *BadgerStore, txs ...*common.VersionedTransaction) {
	t.Helper()
	for _, tx := range txs {
		batchIntegrityRequireTransactionMissing(t, store, tx)
		batchIntegrityRequireGhostMissing(t, store, tx)
	}
}

func TestBatchStorageRollback(t *testing.T) {
	t.Run("input conflict", func(t *testing.T) {
		store := newTestBadgerStore(t)
		_, utxo := batchIntegrityFunding(1)
		batchIntegrityPutUTXO(t, store, utxo)

		deposit := batchIntegrityDeposit("rollback-input-deposit", 2, common.XINAsset.AssetKey)
		mint := batchIntegrityMint(1001, 3)
		firstSpend := batchIntegritySpend(utxo, 4)
		secondSpend := batchIntegritySpend(utxo, 5)
		txs := []*common.VersionedTransaction{deposit, mint, firstSpend, secondSpend}

		err := store.LockAndPersistTransactions(txs, false)
		require.ErrorContains(t, err, "utxo locked")

		depositOwner, err := store.ReadDepositLock(deposit.Inputs[0].Deposit)
		require.NoError(t, err)
		require.False(t, depositOwner.HasValue())
		batchIntegrityRequireMissing(t, store, graphMintKey(mint.Inputs[0].Mint.Batch))
		persistedUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
		require.NoError(t, err)
		require.False(t, persistedUTXO.LockHash.HasValue())
		batchIntegrityRequireNoNewRecords(t, store, txs...)
	})

	t.Run("ghost conflict", func(t *testing.T) {
		store := newTestBadgerStore(t)
		_, utxo := batchIntegrityFunding(11)
		batchIntegrityPutUTXO(t, store, utxo)

		deposit := batchIntegrityDeposit("rollback-ghost-deposit", 12, common.XINAsset.AssetKey)
		mint := batchIntegrityMint(1011, 13)
		spend := batchIntegritySpend(utxo, 12)
		require.Equal(t, *deposit.Outputs[0].Keys[0], *spend.Outputs[0].Keys[0])
		txs := []*common.VersionedTransaction{deposit, mint, spend}

		err := store.LockAndPersistTransactions(txs, false)
		require.ErrorContains(t, err, "duplicated ghost key")

		depositOwner, err := store.ReadDepositLock(deposit.Inputs[0].Deposit)
		require.NoError(t, err)
		require.False(t, depositOwner.HasValue())
		batchIntegrityRequireMissing(t, store, graphMintKey(mint.Inputs[0].Mint.Batch))
		persistedUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
		require.NoError(t, err)
		require.False(t, persistedUTXO.LockHash.HasValue())
		batchIntegrityRequireNoNewRecords(t, store, txs...)
	})

	t.Run("transaction body failure", func(t *testing.T) {
		store := newTestBadgerStore(t)
		batchIntegrityPutAsset(t, store)
		_, utxo := batchIntegrityFunding(21)
		batchIntegrityPutUTXO(t, store, utxo)

		deposit := batchIntegrityDeposit("rollback-body-deposit", 22, common.XINAsset.AssetKey)
		mint := batchIntegrityMint(1021, 23)
		spend := batchIntegritySpend(utxo, 24)
		invalid := batchIntegrityDeposit("rollback-body-invalid", 25, "incompatible-asset-key")
		txs := []*common.VersionedTransaction{deposit, mint, spend, invalid}

		err := store.LockAndPersistTransactions(txs, false)
		require.ErrorContains(t, err, "invalid asset info")

		for _, tx := range []*common.VersionedTransaction{deposit, invalid} {
			owner, err := store.ReadDepositLock(tx.Inputs[0].Deposit)
			require.NoError(t, err)
			require.False(t, owner.HasValue())
		}
		batchIntegrityRequireMissing(t, store, graphMintKey(mint.Inputs[0].Mint.Batch))
		persistedUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
		require.NoError(t, err)
		require.False(t, persistedUTXO.LockHash.HasValue())
		batchIntegrityRequireNoNewRecords(t, store, txs...)
	})

	t.Run("failed fork replacement", func(t *testing.T) {
		store := newTestBadgerStore(t)
		batchIntegrityPutAsset(t, store)
		_, utxo := batchIntegrityFunding(31)
		batchIntegrityPutUTXO(t, store, utxo)

		priorSpend := batchIntegritySpend(utxo, 32)
		priorDeposit := batchIntegrityDeposit("rollback-fork-deposit", 33, common.XINAsset.AssetKey)
		priorMint := batchIntegrityMint(1031, 34)
		prior := []*common.VersionedTransaction{priorSpend, priorDeposit, priorMint}
		require.NoError(t, store.LockAndPersistTransactions(prior, false))

		replacementSpend := batchIntegritySpend(utxo, 35)
		replacementDeposit := batchIntegrityDeposit("rollback-fork-deposit", 36, common.XINAsset.AssetKey)
		replacementMint := batchIntegrityMint(1031, 37)
		invalid := batchIntegrityDeposit("rollback-fork-invalid", 38, "incompatible-asset-key")
		replacements := []*common.VersionedTransaction{
			replacementSpend,
			replacementDeposit,
			replacementMint,
			invalid,
		}

		err := store.LockAndPersistTransactions(replacements, true)
		require.ErrorContains(t, err, "invalid asset info")

		persistedUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
		require.NoError(t, err)
		require.Equal(t, priorSpend.PayloadHash(), persistedUTXO.LockHash)
		depositOwner, err := store.ReadDepositLock(priorDeposit.Inputs[0].Deposit)
		require.NoError(t, err)
		require.Equal(t, priorDeposit.PayloadHash(), depositOwner)
		require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
			distribution, err := readMintInput(txn, priorMint.Inputs[0].Mint)
			if err != nil {
				return err
			}
			require.Equal(t, priorMint.PayloadHash(), distribution.Transaction)
			require.Zero(t, distribution.Amount.Cmp(priorMint.Inputs[0].Mint.Amount))
			return nil
		}))

		for _, tx := range prior {
			batchIntegrityRequireTransaction(t, store, tx)
			for _, output := range tx.Outputs {
				for _, key := range output.Keys {
					owner, err := store.ReadGhostKeyLock(*key)
					require.NoError(t, err)
					require.NotNil(t, owner)
					require.Equal(t, tx.PayloadHash(), *owner)
				}
			}
		}
		invalidOwner, err := store.ReadDepositLock(invalid.Inputs[0].Deposit)
		require.NoError(t, err)
		require.False(t, invalidOwner.HasValue())
		batchIntegrityRequireNoNewRecords(t, store, replacements...)
	})

	t.Run("badger transaction limit", func(t *testing.T) {
		store := newTestBadgerStore(t)
		require.NoError(t, store.snapshotsDB.Close())
		store.snapshotsDB = openBadgerWithBatchCount(t, 6)

		first := batchIntegrityMint(1041, 41)
		second := batchIntegrityMint(1042, 42)
		txs := []*common.VersionedTransaction{first, second}
		err := store.LockAndPersistTransactions(txs, false)
		require.ErrorIs(t, err, badger.ErrTxnTooBig)

		for _, tx := range txs {
			batchIntegrityRequireMissing(t, store, graphMintKey(tx.Inputs[0].Mint.Batch))
		}
		batchIntegrityRequireNoNewRecords(t, store, txs...)
	})
}

func TestBatchForkPreservesFinalizedOwners(t *testing.T) {
	finalize := func(t *testing.T, store *BadgerStore, tx *common.VersionedTransaction) {
		t.Helper()
		require.NoError(t, store.snapshotsDB.Update(func(dbTxn *badger.Txn) error {
			hash := tx.PayloadHash()
			snapshot := crypto.Blake3Hash(append([]byte("finalized owner"), hash[:]...))
			return dbTxn.Set(graphFinalizationKey(hash), snapshot[:])
		}))
	}

	t.Run("utxo", func(t *testing.T) {
		store := newTestBadgerStore(t)
		_, utxo := batchIntegrityFunding(51)
		batchIntegrityPutUTXO(t, store, utxo)
		prior := batchIntegritySpend(utxo, 52)
		require.NoError(t, store.LockAndPersistTransactions([]*common.VersionedTransaction{prior}, false))
		finalize(t, store, prior)

		replacement := batchIntegritySpend(utxo, 53)
		err := store.LockAndPersistTransactions([]*common.VersionedTransaction{replacement}, true)
		require.ErrorContains(t, err, "prune finalized transaction")

		persistedUTXO, err := store.ReadUTXOLock(utxo.Hash, utxo.Index)
		require.NoError(t, err)
		require.Equal(t, prior.PayloadHash(), persistedUTXO.LockHash)
		batchIntegrityRequireFinalizedTransaction(t, store, prior)
		batchIntegrityRequireNoNewRecords(t, store, replacement)
	})

	t.Run("deposit", func(t *testing.T) {
		store := newTestBadgerStore(t)
		batchIntegrityPutAsset(t, store)
		prior := batchIntegrityDeposit("finalized-deposit-owner", 61, common.XINAsset.AssetKey)
		require.NoError(t, store.LockAndPersistTransactions([]*common.VersionedTransaction{prior}, false))
		finalize(t, store, prior)

		replacement := batchIntegrityDeposit("finalized-deposit-owner", 62, common.XINAsset.AssetKey)
		err := store.LockAndPersistTransactions([]*common.VersionedTransaction{replacement}, true)
		require.ErrorContains(t, err, "prune finalized transaction")

		owner, err := store.ReadDepositLock(prior.Inputs[0].Deposit)
		require.NoError(t, err)
		require.Equal(t, prior.PayloadHash(), owner)
		batchIntegrityRequireFinalizedTransaction(t, store, prior)
		batchIntegrityRequireNoNewRecords(t, store, replacement)
	})

	t.Run("mint", func(t *testing.T) {
		store := newTestBadgerStore(t)
		prior := batchIntegrityMint(1071, 71)
		require.NoError(t, store.LockAndPersistTransactions([]*common.VersionedTransaction{prior}, false))
		finalize(t, store, prior)

		replacement := batchIntegrityMint(1071, 72)
		err := store.LockAndPersistTransactions([]*common.VersionedTransaction{replacement}, true)
		require.ErrorContains(t, err, "prune finalized transaction")

		require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
			distribution, err := readMintInput(txn, prior.Inputs[0].Mint)
			if err != nil {
				return err
			}
			require.Equal(t, prior.PayloadHash(), distribution.Transaction)
			return nil
		}))
		batchIntegrityRequireFinalizedTransaction(t, store, prior)
		batchIntegrityRequireNoNewRecords(t, store, replacement)
	})
}
