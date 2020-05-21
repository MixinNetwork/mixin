package crypto

import "golang.org/x/crypto/sha3"

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
		CosiDumps(cosi *CosiSignature) (data []byte, err error)
		CosiLoads(cosi *CosiSignature, data []byte) (rest []byte, err error)
		CosiLoadCommitents(cosi *CosiSignature, commitents map[int]PublicKey) error
		CosiChallenge(cosi *CosiSignature, publics map[int]PublicKey, message []byte) ([32]byte, error)
		CosiAggregateSignature(cosi *CosiSignature, node int, sig Signature) error
		DumpSignatureResponse(sig Signature) []byte
		LoadSignatureResponse(cosi *CosiSignature, data []byte) (Signature, error)
		CosiFullVerify(publics map[int]PublicKey, message []byte, sig CosiSignature) bool
	}
)

var (
	keyFactory KeyFactory
	hashFunc   HashFunc = sha3.Sum256
)

func SetKeyFactory(f KeyFactory) {
	keyFactory = f
}

func SetHashFunc(h HashFunc) {
	hashFunc = h
}
