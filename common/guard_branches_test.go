package common

import (
	"testing"

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
