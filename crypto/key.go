package crypto

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/crypto/edwards25519"
)

type Key [32]byte

func NewKeyFromSeed(seed []byte) Key {
	var key [32]byte
	var src [64]byte
	copy(src[:], seed)
	edwards25519.ScReduce(&key, &src)
	return key
}

func (k Key) Public() Key {
	var point edwards25519.ExtendedGroupElement
	tmp := [32]byte(k)
	edwards25519.GeScalarMultBase(&point, &tmp)
	point.ToBytes(&tmp)
	return tmp
}

func KeyMult(pub, priv *Key) *Key {
	var point edwards25519.ExtendedGroupElement
	var point2 edwards25519.ProjectiveGroupElement

	tmp := [32]byte(*pub)
	point.FromBytes(&tmp)
	tmp = [32]byte(*priv)
	edwards25519.GeScalarMult(&point2, &tmp, &point)

	point2.ToBytes(&tmp)
	key := Key(tmp)
	return &key
}

func DeriveGhostPublicKey(r, A, B *Key) *Key {
	var point1, point2 edwards25519.ExtendedGroupElement
	var point3 edwards25519.CachedGroupElement
	var point4 edwards25519.CompletedGroupElement
	var point5 edwards25519.ProjectiveGroupElement

	tmp := [32]byte(*B)
	point1.FromBytes(&tmp)
	scalar := KeyMult(A, r).HashScalar()
	edwards25519.GeScalarMultBase(&point2, scalar)
	point2.ToCached(&point3)
	edwards25519.GeAdd(&point4, &point1, &point3)
	point4.ToProjective(&point5)
	point5.ToBytes(&tmp)
	key := Key(tmp)
	return &key
}

func DeriveGhostPrivateKey(R, a, b *Key) *Key {
	scalar := KeyMult(R, a).HashScalar()
	tmp := [32]byte(*b)
	edwards25519.ScAdd(&tmp, &tmp, scalar)
	key := Key(tmp)
	return &key
}

func ViewGhostOutputKey(P, a, R *Key) *Key {
	var point1, point2 edwards25519.ExtendedGroupElement
	var point3 edwards25519.CachedGroupElement
	var point4 edwards25519.CompletedGroupElement
	var point5 edwards25519.ProjectiveGroupElement

	tmp := [32]byte(*P)
	point1.FromBytes(&tmp)
	scalar := KeyMult(R, a).HashScalar()
	edwards25519.GeScalarMultBase(&point2, scalar)
	point2.ToCached(&point3)
	edwards25519.GeSub(&point4, &point1, &point3)
	point4.ToProjective(&point5)
	point5.ToBytes(&tmp)
	key := Key(tmp)
	return &key
}

func (k Key) HashScalar() *[32]byte {
	var out [32]byte
	var src [64]byte
	hash := NewHash(k[:])
	copy(src[:32], hash[:])
	hash = NewHash(hash[:])
	copy(src[32:], hash[:])
	edwards25519.ScReduce(&out, &src)
	return &out
}

func (k Key) String() string {
	return hex.EncodeToString(k[:])
}

func (k Key) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(k.String())), nil
}

func (k *Key) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(k) {
		return fmt.Errorf("invalid key length %d", len(data))
	}
	copy(k[:], data)
	return nil
}
