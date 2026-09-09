package common

import (
	"bytes"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestIntegerRejectsInvalidOperands(t *testing.T) {
	require.Panics(t, func() { NewIntegerFromString("not-a-number") })
	require.Panics(t, func() { NewIntegerFromString("-1") })
	require.Panics(t, func() { NewInteger(1).Add(Zero) })
	require.Panics(t, func() { NewInteger(1).Sub(Zero) })
	require.Panics(t, func() { NewInteger(1).Sub(NewInteger(2)) })
	require.Panics(t, func() { NewInteger(1).Mul(0) })
	require.Panics(t, func() { NewInteger(1).Div(0) })
	require.Panics(t, func() { Zero.Count(NewInteger(1)) })
	require.Panics(t, func() {
		NewIntegerFromString("18446744073709551616").Count(NewInteger(1))
	})

	var integer Integer
	require.Error(t, integer.UnmarshalJSON([]byte("not-json")))
}

func TestEncoderRejectsInvalidStructures(t *testing.T) {
	txHash := crypto.Blake3Hash([]byte("encoder guard transaction"))

	require.Panics(t, func() {
		NewEncoder().EncodeSnapshotPayload(&Snapshot{Version: SnapshotVersionCommonEncoding - 1})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeSnapshotPayload(&Snapshot{Version: SnapshotVersionCommonEncoding})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeSnapshotPayload(&Snapshot{Version: SnapshotVersionCommonEncoding, RoundNumber: 1})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeSnapshotPayload(&Snapshot{
			Version:      SnapshotVersionCommonEncoding,
			Transactions: []crypto.Hash{txHash},
			Signature:    &crypto.CosiSignature{Mask: 1},
		})
	})

	require.Panics(t, func() {
		NewEncoder().EncodeTransaction(&SignedTransaction{})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeTransaction(&SignedTransaction{Transaction: Transaction{
			Version: TxVersionHashSignature,
			Inputs:  make([]*Input, SliceCountLimit+1),
		}})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeTransaction(&SignedTransaction{Transaction: Transaction{
			Version: TxVersionHashSignature,
			Outputs: make([]*Output, SliceCountLimit+1),
		}})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeTransaction(&SignedTransaction{Transaction: Transaction{
			Version: TxVersionHashSignature,
			Extra:   make([]byte, ExtraSizeStorageCapacity+1),
		}})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeTransaction(&SignedTransaction{
			Transaction:   Transaction{Version: TxVersionHashSignature},
			SignaturesMap: make([]map[uint16]*crypto.Signature, MaximumEncodingInt),
		})
	})
	require.Panics(t, func() {
		NewEncoder().EncodeInput(&Input{Index: InputIndexLimit + 1})
	})
}

func TestScriptRejectsInvalidThresholdAndJSON(t *testing.T) {
	require.Error(t, NewThresholdScript(Operator64+1).VerifyFormat())

	var script Script
	require.Error(t, script.UnmarshalJSON([]byte("not-json")))
	require.Error(t, script.UnmarshalJSON([]byte(`"zz"`)))
}

func TestValidateRejectsNodeRemoveWithoutSignatureMap(t *testing.T) {
	seed := make([]byte, 64)
	crypto.ReadRand(seed)
	accounts := make([]*Address, 4)
	for i := range accounts {
		s := make([]byte, 64)
		s[i] = byte(i + 1)
		a := NewAddressFromSeed(s)
		accounts[i] = &a
	}
	store := storeImpl{seed: seed, accounts: accounts}
	mask := crypto.NewKeyFromSeed(bytes.Repeat([]byte{1}, 64)).Public()

	ver := NewTransactionV5(XINAssetId).AsVersioned()
	ver.AddInput(crypto.Blake3Hash([]byte("node remove panic input")), 0)
	ver.Outputs = append(ver.Outputs, &Output{
		Type:   OutputTypeNodeRemove,
		Amount: NewInteger(10000),
		Script: Script{OperatorCmp, OperatorSum, 1},
		Mask:   mask,
	})
	require.Equal(t, uint8(TransactionTypeNodeRemove), ver.TransactionType())

	err := ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.ErrorContains(t, err, "invalid signature map count 0 0")

	ver.AddInput(crypto.Blake3Hash([]byte("node remove panic input 2")), 0)
	ver.resetCache()
	ver.SignaturesMap = []map[uint16]*crypto.Signature{{0: &crypto.Signature{}}}
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.ErrorContains(t, err, "invalid signature map count 1 1")
}
