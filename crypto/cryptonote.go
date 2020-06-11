package crypto

func DeriveGhostPublicKey(r PrivateKey, A, B PublicKey, outputIndex uint64) PublicKey {
	return r.ScalarMult(A).ScalarHash(outputIndex).Public().AddPublic(B)
}

func DeriveGhostPrivateKey(R PublicKey, a, b PrivateKey, outputIndex uint64) PrivateKey {
	return a.ScalarMult(R).ScalarHash(outputIndex).AddPrivate(b)
}

func ViewGhostOutputKey(R, P PublicKey, a PrivateKey, outputIndex uint64) PublicKey {
	return P.SubPublic(a.ScalarMult(R).ScalarHash(outputIndex).Public())
}
