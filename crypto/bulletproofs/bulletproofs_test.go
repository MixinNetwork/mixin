package bulletproofs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

type scalarSequenceReader struct {
	next byte
}

func newScalarSequenceReader() io.Reader {
	return &scalarSequenceReader{next: 1}
}

func (reader *scalarSequenceReader) Read(dst []byte) (int, error) {
	clear(dst)
	for offset := 0; offset < len(dst); offset += 32 {
		dst[offset] = reader.next
		reader.next++
		if reader.next == 0 {
			reader.next = 1
		}
	}
	return len(dst), nil
}

func testBlind(value byte) Scalar {
	var blind Scalar
	blind[0] = value
	return blind
}

func TestMoneroValidBoundaries(t *testing.T) {
	for _, value := range []uint64{0, 1, math.MaxUint64} {
		proof, commitments, err := Prove(
			[]uint64{value}, []Scalar{testBlind(42)}, newScalarSequenceReader(),
		)
		require.NoError(t, err)
		require.True(t, proof.Verify(commitments))
	}
}

func TestMoneroValidAggregated(t *testing.T) {
	// These SHA-256 digests were independently produced by P2Pool consensus
	// v5.0.10 for the same witnesses and scalar stream. Monero's own unit test
	// generates proofs at run time rather than publishing serialized fixtures.
	wantProofDigests := [...]string{
		"6fdd03c9d8e5b172aed9a0316fb8f84eee0daf97638d5db201fec999ec843939",
		"61d66d093f38743c496ad4757eee7eb2393a99a61645d5c6fa3017a6ebf57f6b",
		"4f1cc4d82fa1fbd4ff5270753c3cf1f0a5ea9c5261907d898023e39f1746be25",
		"7b58cbf50880db341079116198a44d74702d3f674ffda87156c46eb68cbf7e67",
		"5a5ead4aa6f6e88a4f0c87d5e0c6b1b34c67180c66f3267c0ad17f53543a0bba",
		"eec0ef65f994f7bb3fd1996a7d01afeacc292d52111ad865478b373fa4bcef9b",
		"21a0ce325e0f1060cc282d425b09a6337ee4009f65585619b60b548e409395ab",
		"2e009cc58ba398002d4827d09f5d393939cf9d2cec362bd17fe0c097e47324fb",
		"5a8d24c4f68d701da60217f28c72c2bb4d543e000654a248aae769a7b2aa5668",
		"85b9230c03845103540e936c4f216ba7e3714e0d587731881aab87a381be5c45",
		"a2bb438c439120c3cd101a524c770deb4e60eae19fb4ffb0d61ef13508c7cbfd",
		"14c1bc86d32ebaa2488d1121fe9488bf6a7bdc2dbd6fe5404e35ed1a903eecbf",
		"2eb6e5e396cf76cab6c36fb304987a96f36f26439539cf1e08d4f3348effd97d",
		"93b941959aa3b30f98bac8dc8b2773f16e3183eb09e37b7465aa423eaeefe2b3",
		"9c6948c566e391c2f84b1f3b9fbfd1754b0e1c70c1f8a48bc4d1b30a89b63acd",
		"9fe1c7da23b23197e71d3ca7137a2a40e04b550d5851c51064a9966ad271422b",
	}
	for count := 1; count <= MaxCommitments; count++ {
		values := make([]uint64, count)
		blindings := make([]Scalar, count)
		for i := range values {
			values[i] = uint64(i+1)*0x01020304050607 + uint64(count)
			blindings[i] = testBlind(byte(i + 17))
		}
		proof, commitments, err := Prove(values, blindings, newScalarSequenceReader())
		require.NoError(t, err, "count=%d", count)
		require.True(t, proof.Verify(commitments), "count=%d", count)
		require.Len(t, proof.L, minRounds+bitsForPaddedCount(count))
		encoded, err := proof.MarshalBinary()
		require.NoError(t, err)
		digest := sha256.Sum256(encoded)
		require.Equal(t, wantProofDigests[count-1], hex.EncodeToString(digest[:]), "count=%d", count)
	}
}

func bitsForPaddedCount(count int) int {
	bits := 0
	for n := 1; n < count; n <<= 1 {
		bits++
	}
	return bits
}

func TestWrongCommitmentRejected(t *testing.T) {
	proof, commitments, err := Prove(
		[]uint64{7, 11}, []Scalar{testBlind(3), testBlind(4)}, newScalarSequenceReader(),
	)
	require.NoError(t, err)
	require.True(t, proof.Verify(commitments))

	wrong, err := Commit(12, testBlind(4))
	require.NoError(t, err)
	commitments[1] = wrong
	require.False(t, proof.Verify(commitments))
}

func TestMoneroTorsionMutationsRejected(t *testing.T) {
	proof, commitments, err := Prove(
		[]uint64{7329838943733}, []Scalar{testBlind(9)}, newScalarSequenceReader(),
	)
	require.NoError(t, err)
	require.True(t, proof.Verify(commitments))

	torsionHex := []string{
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac03fa",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85",
		"ecffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f",
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05",
		"0000000000000000000000000000000000000000000000000000000000000080",
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a",
	}
	for _, encoded := range torsionHex {
		torsionBytes, err := hex.DecodeString(encoded)
		require.NoError(t, err)
		torsion, err := edwards25519.NewIdentityPoint().SetBytes(torsionBytes)
		require.NoError(t, err)

		mutate := func(point EncodedPoint) EncodedPoint {
			decoded, err := decodeCanonicalPoint(point)
			require.NoError(t, err)
			return encodePoint(edwards25519.NewIdentityPoint().Add(decoded, torsion))
		}

		mutatedCommitments := append([]Commitment(nil), commitments...)
		mutatedCommitments[0] = Commitment(mutate(EncodedPoint(mutatedCommitments[0])))
		require.False(t, proof.Verify(mutatedCommitments), "%s/V[0]", encoded)

		for _, target := range []string{"A", "A1", "B"} {
			mutated := cloneProof(proof)
			switch target {
			case "A":
				mutated.A = mutate(mutated.A)
			case "A1":
				mutated.A1 = mutate(mutated.A1)
			case "B":
				mutated.B = mutate(mutated.B)
			}
			require.False(t, mutated.Verify(commitments), "%s/%s", encoded, target)
		}
		for i := range proof.L {
			mutated := cloneProof(proof)
			mutated.L[i] = mutate(mutated.L[i])
			require.False(t, mutated.Verify(commitments), "%s/L[%d]", encoded, i)
		}
		for i := range proof.R {
			mutated := cloneProof(proof)
			mutated.R[i] = mutate(mutated.R[i])
			require.False(t, mutated.Verify(commitments), "%s/R[%d]", encoded, i)
		}
	}
}

func TestMoneroRandomScalarRejectionSampling(t *testing.T) {
	// Monero rejects values at or above 15*l and zero after reduction.
	var zero [32]byte
	var one [32]byte
	one[0] = 1
	random := io.MultiReader(
		bytes.NewReader(scalarLimit[:]),
		bytes.NewReader(zero[:]),
		bytes.NewReader(one[:]),
	)
	got, err := RandomScalar(random)
	require.NoError(t, err)
	require.Equal(t, testBlind(1), got)
}

func TestProofEncodingStrictRoundTrip(t *testing.T) {
	proof, commitments, err := Prove(
		[]uint64{0, 5, math.MaxUint64},
		[]Scalar{testBlind(1), testBlind(2), testBlind(3)},
		newScalarSequenceReader(),
	)
	require.NoError(t, err)
	encoded, err := proof.MarshalBinary()
	require.NoError(t, err)

	var decoded Proof
	require.NoError(t, decoded.UnmarshalBinary(encoded))
	require.Equal(t, proof, &decoded)
	require.True(t, decoded.Verify(commitments))

	for end := range encoded {
		var truncated Proof
		require.Error(t, truncated.UnmarshalBinary(encoded[:end]), "end=%d", end)
	}
	var trailing Proof
	require.ErrorIs(t, trailing.UnmarshalBinary(append(encoded, 0)), ErrTrailingData)

	// The first vector length is at byte 192; 0x86 0x00 is a non-canonical
	// encoding of six.
	nonCanonical := append([]byte(nil), encoded[:192]...)
	nonCanonical = append(nonCanonical, 0x86, 0x00)
	nonCanonical = append(nonCanonical, encoded[193:]...)
	var rejected Proof
	require.Error(t, rejected.UnmarshalBinary(nonCanonical))

	reencoded, err := decoded.MarshalBinary()
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoded, reencoded))
}

func TestBatchVerification(t *testing.T) {
	var items []BatchItem
	for count := 1; count <= 4; count++ {
		values := make([]uint64, count)
		blindings := make([]Scalar, count)
		for i := range values {
			values[i] = uint64(count*100 + i)
			blindings[i] = testBlind(byte(count*10 + i + 1))
		}
		proof, commitments, err := Prove(values, blindings, newScalarSequenceReader())
		require.NoError(t, err)
		items = append(items, BatchItem{Proof: proof, Commitments: commitments})
	}
	valid, err := verifyBatch(items, newScalarSequenceReader())
	require.NoError(t, err)
	require.True(t, valid)

	items[2].Proof = cloneProof(items[2].Proof)
	items[2].Proof.R1[0] ^= 1
	valid, err = verifyBatch(items, newScalarSequenceReader())
	require.NoError(t, err)
	require.False(t, valid)
}

func TestInputValidation(t *testing.T) {
	_, _, err := Prove(nil, nil, newScalarSequenceReader())
	require.ErrorIs(t, err, ErrEmptyStatement)
	_, _, err = Prove([]uint64{1}, nil, newScalarSequenceReader())
	require.ErrorIs(t, err, ErrMismatchedWitness)
	_, _, err = Prove(make([]uint64, MaxCommitments+1), make([]Scalar, MaxCommitments+1), newScalarSequenceReader())
	require.ErrorIs(t, err, ErrTooManyCommitments)

	nonCanonical := Scalar{
		0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58,
		0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
	}
	_, err = Commit(1, nonCanonical)
	require.ErrorIs(t, err, ErrInvalidScalar)
	_, _, err = Prove([]uint64{1}, []Scalar{nonCanonical}, newScalarSequenceReader())
	require.ErrorIs(t, err, ErrInvalidScalar)

	failingReader := io.MultiReader(bytes.NewReader(make([]byte, 7)), errorReader{})
	_, _, err = Prove([]uint64{1}, []Scalar{testBlind(1)}, failingReader)
	require.Error(t, err)

	_, err = VerifyBatch(nil)
	require.ErrorIs(t, err, ErrEmptyStatement)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("injected reader failure")
}

func FuzzProofUnmarshal(f *testing.F) {
	proof, _, err := Prove(
		[]uint64{0, 1, math.MaxUint64},
		[]Scalar{testBlind(1), testBlind(2), testBlind(3)},
		newScalarSequenceReader(),
	)
	if err != nil {
		f.Fatal(err)
	}
	valid, err := proof.MarshalBinary()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0xff}, 512))
	f.Fuzz(func(t *testing.T, input []byte) {
		var parsed Proof
		if err := parsed.UnmarshalBinary(input); err != nil {
			return
		}
		reencoded, err := parsed.MarshalBinary()
		if err != nil {
			t.Fatalf("accepted proof failed to marshal: %v", err)
		}
		if !bytes.Equal(input, reencoded) {
			t.Fatal("accepted proof did not round-trip canonically")
		}
	})
}

func cloneProof(proof *Proof) *Proof {
	clone := *proof
	clone.L = append([]EncodedPoint(nil), proof.L...)
	clone.R = append([]EncodedPoint(nil), proof.R...)
	return &clone
}

func TestIdentityCommitmentAccepted(t *testing.T) {
	// C = 0*G + 0*H is the identity point. Monero accepts it as a valid
	// commitment, so both single and batch verification must accept its proof.
	var zeroBlind Scalar
	identity, err := Commit(0, zeroBlind)
	require.NoError(t, err)
	_, err = decodeCommitment(identity)
	require.NoError(t, err)

	proof, commitments, err := Prove([]uint64{0}, []Scalar{zeroBlind}, newScalarSequenceReader())
	require.NoError(t, err)
	require.Equal(t, []Commitment{identity}, commitments)
	require.True(t, proof.Verify(commitments))
	valid, err := verifyBatch([]BatchItem{{Proof: proof, Commitments: commitments}}, newScalarSequenceReader())
	require.NoError(t, err)
	require.True(t, valid)
}

func TestProofRoundsMustMatchCommitmentCount(t *testing.T) {
	proof1, commitments1, err := Prove([]uint64{7}, []Scalar{testBlind(7)}, newScalarSequenceReader())
	require.NoError(t, err)
	require.Len(t, proof1.L, minRounds) // one value: 6 rounds

	proof3, commitments3, err := Prove(
		[]uint64{1, 2, 3},
		[]Scalar{testBlind(1), testBlind(2), testBlind(3)},
		newScalarSequenceReader(),
	)
	require.NoError(t, err)
	require.Len(t, proof3.L, minRounds+2) // three values pad to four: 8 rounds

	// A proof must carry exactly the rounds its commitment count implies.
	require.False(t, proof1.Verify(commitments3))
	require.False(t, proof3.Verify(commitments3[:1]))

	// Same check when the proof itself is truncated but the statement is not.
	truncated := cloneProof(proof3)
	truncated.L = truncated.L[:minRounds]
	truncated.R = truncated.R[:minRounds]
	require.False(t, truncated.Verify(commitments3))

	// The batch path applies the same rule.
	valid, err := verifyBatch([]BatchItem{{Proof: proof1, Commitments: commitments3}}, newScalarSequenceReader())
	require.NoError(t, err)
	require.False(t, valid)

	// The honest cases still pass.
	require.True(t, proof1.Verify(commitments1))
	require.True(t, proof3.Verify(commitments3))
}

// TestMoneroMainnetFixture verifies a Bulletproofs+ proof produced by Monero's
// own C++ implementation. It was extracted from the canonical blob of
// transaction d42d9c48790754af2fedfd5137a793378163eefb1c973cddc2fc362a66b5daf1,
// mined in Monero block 3731883; the commitments are the transaction's two
// outPk masks. The proof's wire layout inside a Monero transaction is
// identical to this package's standalone encoding.
func TestMoneroMainnetFixture(t *testing.T) {
	const mainnetProofHex = "28af36c71d6f7137071cab97803fdd2c3163f1f27de859fbbc647abab61b6df7dacacad231c3b3047f4fbf9dca6627a0" +
		"cd74388bb9dc3c5f3cd9012282606de5556113178de02fd31c25ce189d4262ac3ee08db3fbf42de14ee950299a828e86" +
		"4e82a95e4c90340640884940a1391e969a0d8e9c2135510311b2984b5351750da18facf93d8ab76d66ce9efab0ee199c" +
		"9e9c5323d6d8fdf62a6a085c3e9eb2036fdeda483c05874be8d2f9cdf67c6f3b60310581d4b8cda9b0619850b501920a" +
		"07581373e16aea0d32f168543490598abff103700a27d81bd395a76a909db51d256903a6e84c2dbe691e9121e012e2f8" +
		"7077de8c90eed63ece6b44373d0af28f4d8211508708600f16e7a8077c96ebb70d9f0baafcbbb1e3393d4eea6245748b" +
		"1d323e4cd16f495e2e8c12f4c51e23470c615b4a94a6968ff540b6b955287cbe1b2155f88799659aac54ed5e732676dc" +
		"91832817e3e0c29872a58c372fe253c1792c529980c05c97c8129de021c11b360d1957da2a70e47cd223b1a4b2981b71" +
		"8c4aa8504466a59868db12275bb0cefecd47baae721d632e8dcfb786ba6d6a748507736482ed0fe094715197ee29821a" +
		"6c18e311df008d91911980b7f158970550438be7b29c1be61f89778cb2384be20ca25a2f1e178c6173953b9ac8ec0af4" +
		"e5da266582bcfce0bc7c5dc464e9bcddb9d5c8ed0002fea1990845b6d8294b5d0dd4544ada3d796fbd84ede7b4cb420d" +
		"91cd8c29d26440747000df78d435729701f30a34d1b4b2dfd33a45ba366ec8ab071a48e3e4b834679021a447eb976600" +
		"0bc6b556511d831cda46c7be003485b522c2fb8d3869f82bac00a6e4905ea1ad09e808d65c2b203a19161e44fcf7301b" +
		"9e36bcfb35dab352650089eb20dad41032b8"
	const mainnetCommitment0Hex = "f260025c00787c485a3449af7b1136efc9906d1fc3fb89707b00bbd66de11208"
	const mainnetCommitment1Hex = "e69353ed2b4672af536cce501093ccedcbaad7c3d4517f8c386f5c1f7ea0d862"

	proofBytes, err := hex.DecodeString(mainnetProofHex)
	require.NoError(t, err)
	var proof Proof
	require.NoError(t, proof.UnmarshalBinary(proofBytes))
	require.Len(t, proof.L, minRounds+1) // two values: 7 rounds

	var commitments []Commitment
	for _, encoded := range []string{mainnetCommitment0Hex, mainnetCommitment1Hex} {
		data, err := hex.DecodeString(encoded)
		require.NoError(t, err)
		var commitment Commitment
		copy(commitment[:], data)
		commitments = append(commitments, commitment)
	}
	require.True(t, proof.Verify(commitments))

	reencoded, err := proof.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, proofBytes, reencoded)
}

// FuzzVerifyConsistency checks that single-proof verification and one-item
// batch verification agree for every canonical proof and bounded statement.
// The second seed is a distinct valid proof for the same commitments, ensuring
// the target does not mistake randomized proof bytes for a mutation.
func FuzzVerifyConsistency(f *testing.F) {
	proof, commitments, err := Prove(
		[]uint64{0, 1, math.MaxUint64},
		[]Scalar{testBlind(1), testBlind(2), testBlind(3)},
		newScalarSequenceReader(),
	)
	if err != nil {
		f.Fatal(err)
	}
	if !proof.Verify(commitments) {
		f.Fatal("generated seed proof failed verification")
	}
	proofBytes, err := proof.MarshalBinary()
	if err != nil {
		f.Fatal(err)
	}
	commitmentBytes := make([]byte, 0, len(commitments)*32)
	for i := range commitments {
		commitmentBytes = append(commitmentBytes, commitments[i][:]...)
	}
	f.Add(proofBytes, commitmentBytes)

	alternate, alternateCommitments, err := Prove(
		[]uint64{0, 1, math.MaxUint64},
		[]Scalar{testBlind(1), testBlind(2), testBlind(3)},
		&scalarSequenceReader{next: 101},
	)
	if err != nil {
		f.Fatal(err)
	}
	if !alternate.Verify(alternateCommitments) {
		f.Fatal("generated alternate proof failed verification")
	}
	alternateBytes, err := alternate.MarshalBinary()
	if err != nil {
		f.Fatal(err)
	}
	alternateCommitmentBytes := make([]byte, 0, len(alternateCommitments)*32)
	for i := range alternateCommitments {
		alternateCommitmentBytes = append(alternateCommitmentBytes, alternateCommitments[i][:]...)
	}
	if bytes.Equal(proofBytes, alternateBytes) {
		f.Fatal("independently randomized proofs have identical encodings")
	}
	if !bytes.Equal(commitmentBytes, alternateCommitmentBytes) {
		f.Fatal("same witness produced different commitments")
	}
	f.Add(alternateBytes, alternateCommitmentBytes)

	f.Fuzz(func(t *testing.T, fuzzProof, fuzzCommitments []byte) {
		var parsed Proof
		if err := parsed.UnmarshalBinary(fuzzProof); err != nil {
			return
		}
		if len(fuzzCommitments)%32 != 0 || len(fuzzCommitments)/32 > MaxCommitments {
			return
		}
		parsedCommitments := make([]Commitment, len(fuzzCommitments)/32)
		for i := range parsedCommitments {
			copy(parsedCommitments[i][:], fuzzCommitments[i*32:])
		}

		singleValid := parsed.Verify(parsedCommitments)
		weightBytes := testBlind(37)
		batchValid, err := verifyBatch(
			[]BatchItem{{Proof: &parsed, Commitments: parsedCommitments}},
			bytes.NewReader(weightBytes[:]),
		)
		if err != nil {
			t.Fatal(err)
		}
		if batchValid != singleValid {
			t.Fatalf("single and one-item batch verification disagree: single=%v batch=%v", singleValid, batchValid)
		}
	})
}
