package ed25519

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

func (k Key) CheckKey() bool {
	var (
		point edwards25519.ExtendedGroupElement
		kBts  = [32]byte(k)
	)
	return point.FromBytes(&kBts)
}

func (k Key) AddPublic(pub crypto.PublicKey) crypto.PublicKey {
	var (
		kBts = [32]byte(k)
		pBts = [32]byte(pub.Key())

		point1 edwards25519.ExtendedGroupElement
		point2 edwards25519.ExtendedGroupElement
		point3 edwards25519.CachedGroupElement
		point4 edwards25519.CompletedGroupElement
		point5 edwards25519.ProjectiveGroupElement
		out    [32]byte
	)
	point1.FromBytes(&kBts)
	point2.FromBytes(&pBts)
	point2.ToCached(&point3)
	edwards25519.GeAdd(&point4, &point1, &point3)
	point4.ToProjective(&point5)
	point5.ToBytes(&out)
	key := Key(out)
	return &key
}

func (k Key) SubPublic(pub crypto.PublicKey) crypto.PublicKey {
	var (
		kBts = [32]byte(k)
		pBts = [32]byte(pub.Key())

		point1 edwards25519.ExtendedGroupElement
		point2 edwards25519.ExtendedGroupElement
		point3 edwards25519.CachedGroupElement
		point4 edwards25519.CompletedGroupElement
		point5 edwards25519.ProjectiveGroupElement
		out    [32]byte
	)
	point1.FromBytes(&kBts)
	point2.FromBytes(&pBts)
	point2.ToCached(&point3)
	edwards25519.GeSub(&point4, &point1, &point3)
	point4.ToProjective(&point5)
	point5.ToBytes(&out)
	key := Key(out)
	return &key
}

func (k Key) ScalarHash(outputIndex uint64) crypto.PrivateKey {
	var (
		src [64]byte
		key Key
	)

	{
		tmp := make([]byte, 12, 12)
		length := binary.PutUvarint(tmp, outputIndex)
		tmp = tmp[:length]

		var buf bytes.Buffer
		buf.Write(k[:])
		buf.Write(tmp)
		hash := crypto.NewHash(buf.Bytes())
		copy(src[:32], hash[:])
		hash = crypto.NewHash(hash[:])
		copy(src[32:], hash[:])
		key = *NewPrivateKeyFromSeedOrPanic(src[:])
	}

	{
		hash := crypto.NewHash(key[:])
		copy(src[:32], hash[:])
		hash = crypto.NewHash(hash[:])
		copy(src[32:], hash[:])
		var out [32]byte
		edwards25519.ScReduce(&out, &src)
		key = Key(out)
	}
	return &key
}

func (k Key) DeterministicHashDerive() crypto.PrivateKey {
	seed := crypto.NewHash(k[:])
	return NewPrivateKeyFromSeedOrPanic(append(seed[:], seed[:]...))
}

func (k Key) Challenge(R crypto.PublicKey, message []byte) [32]byte {
	var (
		hramDigest        [64]byte
		hramDigestReduced [32]byte
		RBts              = R.Key()
	)

	h := sha512.New()
	h.Write(RBts[:])
	h.Write(k[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	edwards25519.ScReduce(&hramDigestReduced, &hramDigest)
	return hramDigestReduced
}
