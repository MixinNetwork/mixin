package crypto

type (
	HashFunc func(data []byte) (digest [32]byte)

	PublicKey interface {
		String() string
		Key() Key
		AddPublic(p PublicKey) PublicKey
		SubPublic(p PublicKey) PublicKey
		ScalarHash(outputIndex uint64) PrivateKey
		DeterministicHashDerive() PrivateKey
		Challenge(R PublicKey, message []byte) [32]byte
		Verify(message []byte, sig Signature) bool
		VerifyWithChallenge(message []byte, sig Signature, hReduced [32]byte) bool
	}
	PrivateKey interface {
		String() string
		Key() Key
		Public() PublicKey
		AddPrivate(p PrivateKey) PrivateKey
		ScalarMult(pub PublicKey) PublicKey
		Sign(message []byte) Signature
		SignWithChallenge(random PrivateKey, message []byte, hReduced [32]byte) Signature
	}
	KeyFactory interface {
		NewPrivateKeyFromSeed(seed []byte) (PrivateKey, error)
		NewPrivateKeyFromSeedPanic(seed []byte) PrivateKey
		PrivateKeyFromKey(k Key) (PrivateKey, error)
		PublicKeyFromKey(k Key) (PublicKey, error)

		// cosi
		CosiInitLoad(cosi *CosiSignature, commitents map[int]PublicKey) error
		CosiChallenge(cosi *CosiSignature, publics map[int]PublicKey, message []byte) ([32]byte, error)
		CosiAggregateSignatures(cosi *CosiSignature, sigs map[int]Signature) error
		CosiFullVerify(publics map[int]PublicKey, message []byte, sig CosiSignature) bool
	}
)

var (
	keyFactory KeyFactory
	hashFunc   HashFunc
)

func SetKeyFactory(f KeyFactory) {
	keyFactory = f
}

func SetHashFunc(h HashFunc) {
	hashFunc = h
}
