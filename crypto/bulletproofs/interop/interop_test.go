package interop

import (
	"bytes"
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
