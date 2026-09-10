package bulletproofs

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

type moneroRangeFixture struct {
	Name        string   `json:"name"`
	Valid       bool     `json:"valid"`
	Values      []string `json:"values"`
	Blindings   []string `json:"blindings"`
	Commitments []string `json:"commitments"`
	Proof       string   `json:"proof"`
}

func TestMoneroOutOfRangeFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/monero_range_proofs.json")
	require.NoError(t, err)
	var fixtures struct {
		SourceCommit string               `json:"source_commit"`
		Cases        []moneroRangeFixture `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(data, &fixtures))
	require.Equal(t, "4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5", fixtures.SourceCommit)

	// Monero's scalar-input prover can represent these wide amounts. Go's
	// public uint64 prover cannot, so the proof bodies come from the reference
	// implementation unchanged. Both wide values are canonical field scalars.
	type expectation struct {
		count       int
		invalidAt   int
		invalidByte int
	}
	expected := map[string]expectation{
		"valid_zero":         {count: 1, invalidAt: -1},
		"valid_max":          {count: 1, invalidAt: -1},
		"invalid_8":          {count: 1, invalidAt: 0, invalidByte: 8},
		"invalid_31":         {count: 1, invalidAt: 0, invalidByte: 31},
		"invalid_8_padded":   {count: 3, invalidAt: 1, invalidByte: 8},
		"invalid_31_maximum": {count: MaxCommitments, invalidAt: MaxCommitments - 1, invalidByte: 31},
	}
	require.Len(t, fixtures.Cases, len(expected))

	controlProof, controlCommitments, err := Prove(
		[]uint64{0, math.MaxUint64},
		[]Scalar{testBlind(41), testBlind(42)},
		newScalarSequenceReader(),
	)
	require.NoError(t, err)
	require.True(t, controlProof.Verify(controlCommitments))
	control := BatchItem{Proof: controlProof, Commitments: controlCommitments}
	valid, err := VerifyBatch([]BatchItem{control, control})
	require.NoError(t, err)
	require.True(t, valid)

	seen := make(map[string]bool)
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			want, exists := expected[fixture.Name]
			require.True(t, exists, "unexpected reference fixture")
			require.False(t, seen[fixture.Name], "duplicate reference fixture")
			seen[fixture.Name] = true
			require.Equal(t, want.invalidAt < 0, fixture.Valid)
			require.Len(t, fixture.Values, want.count)
			require.Len(t, fixture.Blindings, want.count)
			require.Len(t, fixture.Commitments, want.count)

			commitments := make([]Commitment, want.count)
			for i := range commitments {
				amountBytes, err := hex.DecodeString(fixture.Values[i])
				require.NoError(t, err)
				amount, err := ParseScalar(amountBytes)
				require.NoError(t, err)
				if i == want.invalidAt {
					var wide Scalar
					wide[want.invalidByte] = 1
					require.Equal(t, wide, amount, "wrong out-of-range amount")
				} else {
					require.Equal(t, make([]byte, 24), amount[8:], "control amount exceeds uint64")
				}
				switch fixture.Name {
				case "valid_zero":
					require.Equal(t, Scalar{}, amount)
				case "valid_max":
					require.Equal(t, encodeScalar(scalarFromUint64(math.MaxUint64)), amount)
				}

				blindBytes, err := hex.DecodeString(fixture.Blindings[i])
				require.NoError(t, err)
				blind, err := ParseScalar(blindBytes)
				require.NoError(t, err)
				commitmentBytes, err := hex.DecodeString(fixture.Commitments[i])
				require.NoError(t, err)
				require.Len(t, commitmentBytes, len(commitments[i]))
				copy(commitments[i][:], commitmentBytes)
				point, err := decodeCommitment(commitments[i])
				require.NoError(t, err, "fixture must contain a canonical prime-order commitment")

				// Reconstruct C using the entire field scalar, never a uint64
				// conversion that would silently discard the out-of-range bits.
				amountScalar, err := decodeScalar(amount)
				require.NoError(t, err)
				blindScalar, err := decodeScalar(blind)
				require.NoError(t, err)
				computed := new(edwards25519.Point).ScalarBaseMult(blindScalar)
				computed.Add(computed, new(edwards25519.Point).ScalarMult(amountScalar, loadGenerators().value))
				require.Equal(t, 1, computed.Equal(point), "fixture commitment must match its full-width amount")
			}

			proofBytes, err := hex.DecodeString(fixture.Proof)
			require.NoError(t, err)
			var proof Proof
			require.NoError(t, proof.UnmarshalBinary(proofBytes))
			reencoded, err := proof.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, proofBytes, reencoded)

			item := BatchItem{Proof: &proof, Commitments: commitments}
			// Require successful structural and transcript processing. Negative
			// fixtures must fail the final equation, not an earlier parser guard.
			verifier := new(batchVerifier)
			require.True(t, verifier.accumulate(item, scalarOne))
			require.Equal(t, fixture.Valid, verifier.verify())
			require.Equal(t, fixture.Valid, proof.Verify(commitments))
			valid, err := VerifyBatch([]BatchItem{item})
			require.NoError(t, err)
			require.Equal(t, fixture.Valid, valid)

			for position := range 3 {
				batch := []BatchItem{control, control, control}
				batch[position] = item
				valid, err := VerifyBatch(batch)
				require.NoError(t, err)
				require.Equal(t, fixture.Valid, valid, "reference item at batch position %d", position)
			}
		})
	}
	require.Len(t, seen, len(expected))
}
