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
		Verify(message []byte, sig *Signature) bool
		VerifyWithChallenge(message []byte, sig *Signature, hReduced [32]byte) bool
	}
	PrivateKey interface {
		String() string
		Key() Key
		Public() PublicKey
		AddPrivate(p PrivateKey) PrivateKey
		ScalarMult(pub PublicKey) PublicKey
		Sign(message []byte) (*Signature, error)
		SignWithChallenge(random PrivateKey, message []byte, hReduced [32]byte) (*Signature, error)
	}
	KeyFactory interface {
		PrivateKeyFromSeed(seed []byte) (PrivateKey, error)
		PrivateKeyFromKey(k Key) (PrivateKey, error)
		PublicKeyFromKey(k Key) (PublicKey, error)

		// cosi
		CosiDumps(cosi *CosiSignature) (data []byte)
		CosiLoads(cosi *CosiSignature, data []byte) (rest []byte, err error)
		CosiAggregateCommitments(cosi *CosiSignature, commitments map[int]*Commitment) error
		CosiChallenge(cosi *CosiSignature, publics map[int]PublicKey, message []byte) ([32]byte, error)
		CosiAggregateSignature(cosi *CosiSignature, keyIndex int, sig *Signature) error
		CosiFullVerify(publics map[int]PublicKey, message []byte, sig *CosiSignature) bool

		UpdateSignatureCommitment(sig *Signature, commitment *Commitment)
		DumpSignatureResponse(sig *Signature) *Response
		LoadResponseSignature(cosi *CosiSignature, commitment *Commitment, response *Response) *Signature
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
