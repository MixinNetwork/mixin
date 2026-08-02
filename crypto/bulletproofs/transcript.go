package bulletproofs

import "filippo.io/edwards25519"

func startTranscript(commitments []*edwards25519.Point) *edwards25519.Scalar {
	data := make([]byte, 0, len(commitments)*32)
	for _, commitment := range commitments {
		transmitted := scalePoint(commitment, scalarInvEight)
		data = append(data, transmitted.Bytes()...)
	}
	commitmentHash := hashToScalar(data)
	initial := loadGenerators().initialTranscript
	return hashToScalar(initial[:], commitmentHash.Bytes())
}

func transcriptPoint(transcript *edwards25519.Scalar, point EncodedPoint) *edwards25519.Scalar {
	return hashToScalar(transcript.Bytes(), point[:])
}

func transcriptPoints(transcript *edwards25519.Scalar, left, right EncodedPoint) *edwards25519.Scalar {
	return hashToScalar(transcript.Bytes(), left[:], right[:])
}
