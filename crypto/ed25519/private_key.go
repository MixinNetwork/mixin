package ed25519

import (
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

func (k Key) CheckScalar() bool {
	kBts := [32]byte(k)
	return edwards25519.ScValid(&kBts)
}

func (k Key) Public() crypto.PublicKey {
	var (
		point edwards25519.ExtendedGroupElement
		kBts  = [32]byte(k)
	)
	edwards25519.GeScalarMultBase(&point, &kBts)
	point.ToBytes(&kBts)
	key := Key(kBts)
	return &key
}

func (k Key) AddPrivate(p crypto.PrivateKey) crypto.PrivateKey {
	var (
		kBts = [32]byte(k)
		pBts = [32]byte(p.Key())
		out  [32]byte
	)

	edwards25519.ScAdd(&out, &kBts, &pBts)
	key := Key(out)
	return &key
}

func (k Key) ScalarMult(p crypto.PublicKey) crypto.PublicKey {
	var (
		kBts   = [32]byte(k)
		pBts   = [32]byte(p.Key())
		point  edwards25519.ExtendedGroupElement
		point2 edwards25519.ProjectiveGroupElement
		out    [32]byte
	)

	point.FromBytes(&pBts)
	edwards25519.GeScalarMult(&point2, &kBts, &point)
	point2.ToBytes(&out)
	key := Key(out)
	return &key
}
