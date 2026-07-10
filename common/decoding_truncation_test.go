package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestDecodeTransactionRejectsEveryTruncation(t *testing.T) {
	private := crypto.NewKeyFromSeed(decoderTestSeed(1))
	public := private.Public()
	mask := crypto.NewKeyFromSeed(decoderTestSeed(2)).Public()
	signature := private.Sign(crypto.Blake3Hash([]byte("decoder signature")))
	signed := &SignedTransaction{
		Transaction: Transaction{
			Version: TxVersionHashSignature,
			Asset:   crypto.Blake3Hash([]byte("decoder asset")),
			Inputs: []*Input{{
				Hash:    crypto.Blake3Hash([]byte("decoder input")),
				Index:   7,
				Genesis: []byte("genesis"),
				Deposit: &DepositData{
					Chain:       BitcoinAssetId,
					AssetKey:    "btc",
					Transaction: "deposit transaction",
					Index:       11,
					Amount:      NewInteger(2),
				},
				Mint: &MintData{Group: mintGroupUniversal, Batch: 9, Amount: NewInteger(3)},
			}},
			Outputs: []*Output{{
				Type:   OutputTypeWithdrawalSubmit,
				Amount: NewInteger(1),
				Keys:   []*crypto.Key{&public},
				Mask:   mask,
				Script: NewThresholdScript(1),
				Withdrawal: &WithdrawalData{
					Address: "destination",
					Tag:     "memo",
				},
			}},
			References: []crypto.Hash{crypto.Blake3Hash([]byte("decoder reference"))},
			Extra:      []byte("decoder extra"),
		},
		SignaturesMap: []map[uint16]*crypto.Signature{{0: &signature}},
	}

	encoded := NewEncoder().EncodeTransaction(signed)
	decoded, err := NewDecoder(encoded).DecodeTransaction()
	require.NoError(t, err)
	require.Equal(t, signed.Transaction, decoded.Transaction)
	require.Equal(t, signed.SignaturesMap, decoded.SignaturesMap)

	for cut := range len(encoded) {
		_, err := NewDecoder(encoded[:cut]).DecodeTransaction()
		require.Error(t, err, "cut=%d", cut)
	}
}

func TestDecodeAggregatedSignatureTransactionsRejectEveryTruncation(t *testing.T) {
	tests := []struct {
		name    string
		signers []int
	}{
		{name: "ordinary mask", signers: []int{0, 1}},
		{name: "sparse mask", signers: []int{0, 100}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signed := &SignedTransaction{
				Transaction: Transaction{Version: TxVersionHashSignature},
				AggregatedSignature: &AggregatedSignature{
					Signers: test.signers,
				},
			}
			encoded := NewEncoder().EncodeTransaction(signed)
			decoded, err := NewDecoder(encoded).DecodeTransaction()
			require.NoError(t, err)
			require.Equal(t, test.signers, decoded.AggregatedSignature.Signers)

			for cut := range len(encoded) {
				_, err := NewDecoder(encoded[:cut]).DecodeTransaction()
				require.Error(t, err, "cut=%d", cut)
			}
		})
	}
}

func TestDecodeSnapshotTruncationsDoNotPanic(t *testing.T) {
	snapshot := &SnapshotWithTopologicalOrder{
		Snapshot: &Snapshot{
			Version:      SnapshotVersionCommonEncoding,
			NodeId:       crypto.Blake3Hash([]byte("decoder node")),
			References:   &RoundLink{Self: crypto.Blake3Hash([]byte("self")), External: crypto.Blake3Hash([]byte("external"))},
			RoundNumber:  7,
			Timestamp:    11,
			Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("snapshot transaction"))},
			Signature:    &crypto.CosiSignature{Mask: 1},
		},
		TopologicalOrder: 13,
	}
	encoded := snapshot.VersionedMarshal()
	decoded, err := NewDecoder(encoded).DecodeSnapshotWithTopo()
	require.NoError(t, err)
	require.Equal(t, snapshot.TopologicalOrder, decoded.TopologicalOrder)

	for cut := range len(encoded) {
		require.NotPanics(t, func() {
			_, _ = NewDecoder(encoded[:cut]).DecodeSnapshotWithTopo()
		}, "cut=%d", cut)
	}
}

func decoderTestSeed(base byte) []byte {
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = base + byte(i)
	}
	return seed
}
