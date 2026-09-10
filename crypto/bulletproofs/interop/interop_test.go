package interop

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	p2curve "git.gammaspectra.live/P2Pool/consensus/v5/monero/crypto/curve25519"
	p2ringct "git.gammaspectra.live/P2Pool/consensus/v5/monero/crypto/ringct"
	p2plus "git.gammaspectra.live/P2Pool/consensus/v5/monero/crypto/ringct/bulletproofs/plus"
	ours "github.com/MixinNetwork/mixin/crypto/bulletproofs"
)

func randomStream() []byte {
	stream := make([]byte, 96*32)
	for i := range 96 {
		stream[i*32] = byte(i + 1)
	}
	return stream
}

func TestExactP2PoolCompatibility(t *testing.T) {
	var p2Batch p2plus.BatchVerifier[p2curve.VarTimeOperations]
	var ourBatch []ours.BatchItem
	p2BatchRandom := bytes.NewReader(randomStream())
	for count := 1; count <= 16; count++ {
		values := make([]uint64, count)
		blinds := make([]ours.Scalar, count)
		witness := make(p2plus.AggregateRangeWitness, count)
		p2Commitments := make([]p2curve.VarTimePublicKey, count)
		for i := 0; i < count; i++ {
			values[i] = uint64(i+1)*0x01020304050607 + uint64(count)
			blinds[i][0] = byte(i + 17)
			var encoded p2curve.PrivateKeyBytes
			copy(encoded[:], blinds[i][:])
			witness[i] = p2ringct.LazyCommitment{
				Mask:   *encoded.Scalar(),
				Amount: values[i],
			}
			p2Commitments[i] = *p2ringct.CalculateCommitment(
				new(p2curve.VarTimePublicKey), witness[i],
			)
		}

		oursProof, oursCommitments, err := ours.Prove(
			values, blinds, bytes.NewReader(randomStream()),
		)
		if err != nil {
			t.Fatalf("ours prove count=%d: %v", count, err)
		}
		oursBytes, err := oursProof.MarshalBinary()
		if err != nil {
			t.Fatalf("ours marshal count=%d: %v", count, err)
		}

		statement := p2plus.AggregateRangeStatement[p2curve.VarTimeOperations]{
			V: p2Commitments,
		}
		p2Proof, err := statement.Prove(witness, bytes.NewReader(randomStream()))
		if err != nil {
			t.Fatalf("p2pool prove count=%d: %v", count, err)
		}
		p2Bytes, err := p2Proof.AppendBinary(nil, false)
		if err != nil {
			t.Fatalf("p2pool marshal count=%d: %v", count, err)
		}

		for i := range oursCommitments {
			if !bytes.Equal(oursCommitments[i][:], p2Commitments[i].Bytes()) {
				t.Fatalf("commitment mismatch count=%d index=%d", count, i)
			}
		}
		if !bytes.Equal(oursBytes, p2Bytes) {
			t.Fatalf("proof mismatch count=%d\nours=%x\np2=%x", count, oursBytes, p2Bytes)
		}

		var parsedP2 p2plus.AggregateRangeProof[p2curve.VarTimeOperations]
		reader := bytes.NewReader(oursBytes)
		if err := parsedP2.FromReader(reader); err != nil || reader.Len() != 0 {
			t.Fatalf("p2pool parse ours count=%d: %v trailing=%d", count, err, reader.Len())
		}
		if !parsedP2.Verify(p2Commitments, bytes.NewReader(randomStream())) {
			t.Fatalf("p2pool rejected ours count=%d", count)
		}

		var parsedOurs ours.Proof
		if err := parsedOurs.UnmarshalBinary(p2Bytes); err != nil {
			t.Fatalf("ours parse p2pool count=%d: %v", count, err)
		}
		if !parsedOurs.Verify(oursCommitments) {
			t.Fatalf("ours rejected p2pool count=%d", count)
		}
		if !statement.Verify(&p2Batch, &p2Proof, p2BatchRandom) {
			t.Fatalf("p2pool failed to accumulate count=%d", count)
		}
		ourBatch = append(ourBatch, ours.BatchItem{
			Proof:       &parsedOurs,
			Commitments: append([]ours.Commitment(nil), oursCommitments...),
		})
	}
	if !p2Batch.Verify() {
		t.Fatal("p2pool batch verifier rejected the 1-through-16 proof batch")
	}
	valid, err := ours.VerifyBatch(ourBatch)
	if err != nil {
		t.Fatalf("ours batch verify: %v", err)
	}
	if !valid {
		t.Fatal("ours batch verifier rejected the 1-through-16 proof batch")
	}
}

func TestMoneroRangeFixturesCompatibility(t *testing.T) {
	data, err := os.ReadFile("../testdata/monero_range_proofs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		SourceCommit string `json:"source_commit"`
		Cases        []struct {
			Name        string   `json:"name"`
			Valid       bool     `json:"valid"`
			Commitments []string `json:"commitments"`
			Proof       string   `json:"proof"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.SourceCommit != "4f92268d7c16741cfb41e5bbe2aa46cc260a9ea5" {
		t.Fatalf("unexpected Monero source commit %q", fixtures.SourceCommit)
	}
	wantCases := map[string]bool{
		"valid_zero": true, "valid_max": true,
		"invalid_8": false, "invalid_31": false,
		"invalid_8_padded": false, "invalid_31_maximum": false,
	}
	if len(fixtures.Cases) != len(wantCases) {
		t.Fatalf("got %d fixtures, want %d", len(fixtures.Cases), len(wantCases))
	}
	type parsedFixture struct {
		name            string
		valid           bool
		oursProof       ours.Proof
		p2Proof         p2plus.AggregateRangeProof[p2curve.VarTimeOperations]
		oursCommitments []ours.Commitment
		p2Commitments   []p2curve.VarTimePublicKey
	}
	parsed := make([]parsedFixture, len(fixtures.Cases))
	var controls []*parsedFixture
	for i, fixture := range fixtures.Cases {
		want, exists := wantCases[fixture.Name]
		if !exists || fixture.Valid != want {
			t.Fatalf("unexpected, duplicate, or misclassified fixture %q", fixture.Name)
		}
		delete(wantCases, fixture.Name)
		item := &parsed[i]
		item.name, item.valid = fixture.Name, fixture.Valid
		proofBytes, err := hex.DecodeString(fixture.Proof)
		if err != nil {
			t.Fatalf("%s: decode proof: %v", item.name, err)
		}
		if err := item.oursProof.UnmarshalBinary(proofBytes); err != nil {
			t.Fatalf("%s: Mixin parse: %v", item.name, err)
		}
		oursBytes, err := item.oursProof.MarshalBinary()
		if err != nil || !bytes.Equal(oursBytes, proofBytes) {
			t.Fatalf("%s: Mixin canonical round trip: %v", item.name, err)
		}
		reader := bytes.NewReader(proofBytes)
		if err := item.p2Proof.FromReader(reader); err != nil || reader.Len() != 0 {
			t.Fatalf("%s: P2Pool parse: %v, trailing=%d", item.name, err, reader.Len())
		}
		p2Bytes, err := item.p2Proof.AppendBinary(nil, false)
		if err != nil || !bytes.Equal(p2Bytes, proofBytes) {
			t.Fatalf("%s: P2Pool canonical round trip: %v", item.name, err)
		}
		item.oursCommitments = make([]ours.Commitment, len(fixture.Commitments))
		item.p2Commitments = make([]p2curve.VarTimePublicKey, len(fixture.Commitments))
		for j, encoded := range fixture.Commitments {
			commitment, err := hex.DecodeString(encoded)
			if err != nil || len(commitment) != 32 {
				t.Fatalf("%s: decode commitment %d: %v, length=%d", item.name, j, err, len(commitment))
			}
			copy(item.oursCommitments[j][:], commitment)
			if _, err := item.p2Commitments[j].SetBytes(commitment); err != nil {
				t.Fatalf("%s: P2Pool commitment %d: %v", item.name, j, err)
			}
			if !item.p2Commitments[j].IsTorsionFree() {
				t.Fatalf("%s: commitment %d is not prime order", item.name, j)
			}
		}
		if item.valid {
			controls = append(controls, item)
		}
	}
	checkBatch := func(t *testing.T, items []*parsedFixture, want bool) {
		t.Helper()
		var p2Batch p2plus.BatchVerifier[p2curve.VarTimeOperations]
		p2Random := bytes.NewReader(randomStream())
		var oursBatch []ours.BatchItem
		for _, item := range items {
			statement := p2plus.AggregateRangeStatement[p2curve.VarTimeOperations]{V: item.p2Commitments}
			if !statement.Verify(&p2Batch, &item.p2Proof, p2Random) {
				t.Fatalf("%s: P2Pool rejected before evaluating the accumulated equation", item.name)
			}
			oursBatch = append(oursBatch, ours.BatchItem{Proof: &item.oursProof, Commitments: item.oursCommitments})
		}
		if got := p2Batch.Verify(); got != want {
			t.Fatalf("P2Pool batch validity=%t, want %t", got, want)
		}
		if got, err := ours.VerifyBatch(oursBatch); err != nil || got != want {
			t.Fatalf("Mixin batch validity=%t, error=%v, want %t", got, err, want)
		}
	}
	for i := range parsed {
		item := &parsed[i]
		t.Run(item.name, func(t *testing.T) {
			if got := item.oursProof.Verify(item.oursCommitments); got != item.valid {
				t.Fatalf("Mixin validity=%t, want %t", got, item.valid)
			}
			if got := item.p2Proof.Verify(item.p2Commitments, bytes.NewReader(randomStream())); got != item.valid {
				t.Fatalf("P2Pool validity=%t, want %t", got, item.valid)
			}
			checkBatch(t, []*parsedFixture{item}, item.valid)
			if !item.valid {
				for position := 0; position <= len(controls); position++ {
					t.Run(fmt.Sprintf("mixed_position_%d", position), func(t *testing.T) {
						mixed := append([]*parsedFixture(nil), controls[:position]...)
						mixed = append(mixed, item)
						mixed = append(mixed, controls[position:]...)
						checkBatch(t, mixed, false)
					})
				}
			}
		})
	}
	t.Run("valid_controls", func(t *testing.T) {
		checkBatch(t, controls, true)
	})
}
