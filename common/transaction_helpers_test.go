package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransactionValidatedSizeContract(t *testing.T) {
	tx := NewTransactionV5(XINAssetId).AsVersioned()
	require.Panics(t, func() {
		tx.ValidatedSize()
	})

	tx.validatedSize = 123
	require.Equal(t, 123, tx.ValidatedSize())
}

func TestSnapshotBatchableTransactionTypes(t *testing.T) {
	script := NewTransactionV5(XINAssetId).AsVersioned()

	depositTx := NewTransactionV5(XINAssetId)
	depositTx.Inputs = []*Input{{Deposit: &DepositData{}}}
	deposit := depositTx.AsVersioned()

	withdrawalSubmitTx := NewTransactionV5(XINAssetId)
	withdrawalSubmitTx.Outputs = []*Output{{Type: OutputTypeWithdrawalSubmit}}
	withdrawalSubmit := withdrawalSubmitTx.AsVersioned()

	withdrawalClaimTx := NewTransactionV5(XINAssetId)
	withdrawalClaimTx.Outputs = []*Output{{Type: OutputTypeWithdrawalClaim}}
	withdrawalClaim := withdrawalClaimTx.AsVersioned()

	mintTx := NewTransactionV5(XINAssetId)
	mintTx.Inputs = []*Input{{Mint: &MintData{}}}
	mint := mintTx.AsVersioned()

	nodeTx := NewTransactionV5(XINAssetId)
	nodeTx.Outputs = []*Output{{Type: OutputTypeNodePledge}}
	node := nodeTx.AsVersioned()

	unknownTx := NewTransactionV5(XINAssetId)
	unknownTx.Inputs = []*Input{{Genesis: []byte("genesis")}}
	unknown := unknownTx.AsVersioned()

	tests := []struct {
		name      string
		tx        *VersionedTransaction
		typeValue uint8
		batchable bool
	}{
		{name: "script", tx: script, typeValue: TransactionTypeScript, batchable: true},
		{name: "deposit", tx: deposit, typeValue: TransactionTypeDeposit, batchable: true},
		{name: "withdrawal submit", tx: withdrawalSubmit, typeValue: TransactionTypeWithdrawalSubmit, batchable: true},
		{name: "withdrawal claim", tx: withdrawalClaim, typeValue: TransactionTypeWithdrawalClaim, batchable: true},
		{name: "mint", tx: mint, typeValue: TransactionTypeMint, batchable: false},
		{name: "node operation", tx: node, typeValue: TransactionTypeNodePledge, batchable: false},
		{name: "unknown", tx: unknown, typeValue: TransactionTypeUnknown, batchable: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.typeValue, test.tx.TransactionType())
			require.Equal(t, test.batchable, test.tx.IsSnapshotBatchable())
		})
	}
}

func TestSnapshotAddTransactionGuards(t *testing.T) {
	tx := crypto.Blake3Hash([]byte("snapshot transaction"))

	require.Panics(t, func() {
		(&Snapshot{Version: SnapshotVersionCommonEncoding - 1}).AddTransaction(tx)
	})

	snapshot := &Snapshot{Version: SnapshotVersionCommonEncoding}
	snapshot.AddTransaction(tx)
	require.Panics(t, func() {
		snapshot.AddTransaction(tx)
	})

	full := &Snapshot{Version: SnapshotVersionCommonEncoding}
	for i := range SnapshotTransactionsMaximum {
		var hash crypto.Hash
		hash[0] = byte(i)
		full.AddTransaction(hash)
	}
	require.Len(t, full.Transactions, SnapshotTransactionsMaximum)
	require.Panics(t, func() {
		var hash crypto.Hash
		hash[0] = SnapshotTransactionsMaximum
		full.AddTransaction(hash)
	})
}
