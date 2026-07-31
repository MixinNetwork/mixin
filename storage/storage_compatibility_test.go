package storage

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

// v0192StorageFixture was produced by v0.19.2 after separately locking each
// input, locking each output ghost key, writing each transaction, and
// finalizing the deposit transaction.
//
//go:embed testdata/v0.19.2-records.json
var v0192StorageFixture []byte

type storageCompatibilityFixture struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Records []struct {
		Name  string `json:"name"`
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"records"`
}

type storageCompatibilityRecord struct {
	Key   []byte
	Value []byte
}

func loadV0192StorageFixture(t *testing.T) map[string]storageCompatibilityRecord {
	t.Helper()

	var fixture storageCompatibilityFixture
	require.NoError(t, json.Unmarshal(v0192StorageFixture, &fixture))
	require.Equal(t, "v0.19.2", fixture.Version)
	require.Equal(t, "b0c120208a66f47ccd727f38189db35cd91d6fa6", fixture.Commit)

	records := make(map[string]storageCompatibilityRecord, len(fixture.Records))
	for _, record := range fixture.Records {
		key, err := hex.DecodeString(record.Key)
		require.NoError(t, err, record.Name)
		value, err := hex.DecodeString(record.Value)
		require.NoError(t, err, record.Name)
		require.NotContains(t, records, record.Name)
		records[record.Name] = storageCompatibilityRecord{Key: key, Value: value}
	}
	require.Len(t, records, 10)
	return records
}

func compatibilityTransaction(t *testing.T, record storageCompatibilityRecord) *common.VersionedTransaction {
	t.Helper()
	tx, err := common.UnmarshalVersionedTransaction(record.Value)
	require.NoError(t, err)
	return tx
}

func compatibilityReadRaw(t *testing.T, store *BadgerStore, key []byte) []byte {
	t.Helper()

	var value []byte
	require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	}))
	return value
}

func TestV0192StorageRecordsAreReadable(t *testing.T) {
	records := loadV0192StorageFixture(t)
	store := newTestBadgerStore(t)
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		for _, record := range records {
			if err := txn.Set(record.Key, record.Value); err != nil {
				return err
			}
		}
		return nil
	}))

	deposit := compatibilityTransaction(t, records["deposit-transaction"])
	depositHash := deposit.PayloadHash()
	require.Equal(t, graphTransactionKey(depositHash), records["deposit-transaction"].Key)
	require.Equal(t, graphFinalizationKey(depositHash), records["deposit-finalization"].Key)
	require.Equal(t, graphDepositKey(deposit.Inputs[0].Deposit), records["deposit-lock"].Key)
	require.Equal(t, graphGhostKey(*deposit.Outputs[0].Keys[0]), records["deposit-ghost"].Key)
	require.Equal(t, graphUtxoKey(depositHash, 0), records["spent-utxo"].Key)

	persisted, finalizedBy, err := store.ReadTransaction(depositHash)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, deposit.Marshal(), persisted.Marshal())
	var finalization crypto.Hash
	require.Len(t, records["deposit-finalization"].Value, len(finalization))
	copy(finalization[:], records["deposit-finalization"].Value)
	require.Equal(t, finalization.String(), finalizedBy)

	depositOwner, err := store.ReadDepositLock(deposit.Inputs[0].Deposit)
	require.NoError(t, err)
	require.Equal(t, depositHash, depositOwner)
	depositGhostOwner, err := store.ReadGhostKeyLock(*deposit.Outputs[0].Keys[0])
	require.NoError(t, err)
	require.NotNil(t, depositGhostOwner)
	require.Equal(t, depositHash, *depositGhostOwner)

	spend := compatibilityTransaction(t, records["spend-transaction"])
	spendHash := spend.PayloadHash()
	require.Equal(t, graphTransactionKey(spendHash), records["spend-transaction"].Key)
	require.Equal(t, graphGhostKey(*spend.Outputs[0].Keys[0]), records["spend-ghost"].Key)
	persisted, finalizedBy, err = store.ReadTransaction(spendHash)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, spend.Marshal(), persisted.Marshal())
	require.Empty(t, finalizedBy)

	utxo, err := store.ReadUTXOLock(depositHash, 0)
	require.NoError(t, err)
	require.Equal(t, spendHash, utxo.LockHash)
	spendGhostOwner, err := store.ReadGhostKeyLock(*spend.Outputs[0].Keys[0])
	require.NoError(t, err)
	require.NotNil(t, spendGhostOwner)
	require.Equal(t, spendHash, *spendGhostOwner)

	mint := compatibilityTransaction(t, records["mint-transaction"])
	mintHash := mint.PayloadHash()
	require.Equal(t, graphTransactionKey(mintHash), records["mint-transaction"].Key)
	require.Equal(t, graphMintKey(mint.Inputs[0].Mint.Batch), records["mint-lock"].Key)
	require.Equal(t, graphGhostKey(*mint.Outputs[0].Keys[0]), records["mint-ghost"].Key)
	persisted, finalizedBy, err = store.ReadTransaction(mintHash)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, mint.Marshal(), persisted.Marshal())
	require.Empty(t, finalizedBy)

	require.NoError(t, store.snapshotsDB.View(func(txn *badger.Txn) error {
		distribution, err := readMintInput(txn, mint.Inputs[0].Mint)
		if err != nil {
			return err
		}
		require.Equal(t, mint.Inputs[0].Mint.Distribute(mintHash).Marshal(), distribution.Marshal())
		return nil
	}))
	mintGhostOwner, err := store.ReadGhostKeyLock(*mint.Outputs[0].Keys[0])
	require.NoError(t, err)
	require.NotNil(t, mintGhostOwner)
	require.Equal(t, mintHash, *mintGhostOwner)
}

func TestBatchStorageWritesV0192CompatibleRecords(t *testing.T) {
	records := loadV0192StorageFixture(t)
	deposit := compatibilityTransaction(t, records["deposit-transaction"])
	spend := compatibilityTransaction(t, records["spend-transaction"])
	mint := compatibilityTransaction(t, records["mint-transaction"])

	// The old fixture contains the deposit output after the old spend locked
	// it. Seed the same output unlocked, then let the current batch writer
	// produce every lock and transaction record.
	utxo, err := common.UnmarshalUTXO(records["spent-utxo"].Value)
	require.NoError(t, err)
	utxo.LockHash = crypto.Hash{}

	store := newTestBadgerStore(t)
	require.NoError(t, store.snapshotsDB.Update(func(txn *badger.Txn) error {
		if err := writeAssetInfo(txn, deposit.Asset, deposit.Inputs[0].Deposit.Asset()); err != nil {
			return err
		}
		return txn.Set(records["spent-utxo"].Key, utxo.Marshal())
	}))
	require.NoError(t, store.LockAndPersistTransactions(
		[]*common.VersionedTransaction{deposit, spend, mint}, false,
	))

	// Matching the frozen v0.19.2 key and value bytes is the reverse
	// compatibility contract: an old reader observes exactly its native
	// deposit, UTXO, mint, ghost, and transaction encodings.
	for name, expected := range records {
		if name == "deposit-finalization" {
			continue
		}
		require.Equal(t, expected.Value, compatibilityReadRaw(t, store, expected.Key), name)
	}
}
