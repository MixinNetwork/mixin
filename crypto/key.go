package crypto

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"filippo.io/edwards25519"
)

type Key [32]byte

func NewKeyFromSeed(seed []byte) Key {
	var key [32]byte
	s := edwards25519.NewScalar().SetUniformBytes(seed)
	copy(key[:], s.Bytes())
	return key
}

func KeyFromString(s string) (Key, error) {
	var key Key
	b, err := hex.DecodeString(s)
	if err != nil {
		return key, err
	}
	if len(b) != len(key) {
		return key, fmt.Errorf("invalid key size %d", len(b))
	}
	copy(key[:], b)
	return key, nil
}

func (k Key) CheckKey() bool {
	_, err := edwards25519.NewIdentityPoint().SetBytes(k[:])
	return err == nil
}

func (k Key) CheckScalar() bool {
	tmp := [32]byte(k)
	return ScValid(&tmp)
}

func signum(a int64) int64 {
	return a>>63 - ((-a) >> 63)
}

func load4(in []byte) int64 {
	var r int64
	r = int64(in[0])
	r |= int64(in[1]) << 8
	r |= int64(in[2]) << 16
	r |= int64(in[3]) << 24
	return r
}

func ScValid(s *[32]byte) bool {
	s0 := load4(s[:])
	s1 := load4(s[4:])
	s2 := load4(s[8:])
	s3 := load4(s[12:])
	s4 := load4(s[16:])
	s5 := load4(s[20:])
	s6 := load4(s[24:])
	s7 := load4(s[28:])
	return (signum(1559614444-s0)+(signum(1477600026-s1)<<1)+(signum(2734136534-s2)<<2)+(signum(350157278-s3)<<3)+(signum(-s4)<<4)+(signum(-s5)<<5)+(signum(-s6)<<6)+(signum(268435456-s7)<<7))>>8 == 0

}

func (k Key) Public() Key {
	x, err := edwards25519.NewScalar().SetCanonicalBytes(k[:])
	if err != nil {
		panic(k.String())
	}
	v := edwards25519.NewIdentityPoint().ScalarBaseMult(x)
	var tmp Key
	copy(tmp[:], v.Bytes())
	return tmp
}

func (k Key) HasValue() bool {
	zero := Key{}
	return !bytes.Equal(k[:], zero[:])
}

func (k Key) DeterministicHashDerive() Key {
	seed := NewHash(k[:])
	return NewKeyFromSeed(append(seed[:], seed[:]...))
}

func KeyMultPubPriv(pub, priv *Key) *edwards25519.Point {
	q, err := edwards25519.NewIdentityPoint().SetBytes(pub[:])
	if err != nil {
		panic(pub.String())
	}
	if !priv.CheckScalar() {
		panic(priv.String())
	}
	x, err := edwards25519.NewScalar().SetCanonicalBytes(priv[:])
	if err != nil {
		panic(priv.String())
	}

	v := edwards25519.NewIdentityPoint().ScalarMult(x, q)
	return v
}

func KeyAddPub(pub1, pub2 *Key) *Key {
	p, err := edwards25519.NewIdentityPoint().SetBytes(pub1[:])
	if err != nil {
		panic(pub1.String())
	}
	q, err := edwards25519.NewIdentityPoint().SetBytes(pub2[:])
	if err != nil {
		panic(pub2.String())
	}

	v := edwards25519.NewIdentityPoint().Add(p, q)
	var key Key
	copy(key[:], v.Bytes())
	return &key
}

func HashScalar(k *edwards25519.Point, outputIndex uint64) *edwards25519.Scalar {
	tmp := make([]byte, 12)
	length := binary.PutUvarint(tmp, outputIndex)
	tmp = tmp[:length]

	var src [64]byte
	var buf bytes.Buffer
	buf.Write(k.Bytes())
	buf.Write(tmp)
	hash := NewHash(buf.Bytes())
	copy(src[:32], hash[:])
	hash = NewHash(hash[:])
	copy(src[32:], hash[:])
	s := edwards25519.NewScalar().SetUniformBytes(src[:])

	hash = NewHash(s.Bytes())
	copy(src[:32], hash[:])
	hash = NewHash(hash[:])
	copy(src[32:], hash[:])
	x := edwards25519.NewScalar().SetUniformBytes(src[:])
	return x
}

func DeriveGhostPublicKey(r, A, B *Key, outputIndex uint64) *Key {
	x := HashScalar(KeyMultPubPriv(A, r), outputIndex)
	p1, err := edwards25519.NewIdentityPoint().SetBytes(B[:])
	if err != nil {
		panic(B.String())
	}
	p2 := edwards25519.NewIdentityPoint().ScalarBaseMult(x)
	p4 := edwards25519.NewIdentityPoint().Add(p1, p2)
	var key Key
	copy(key[:], p4.Bytes())
	return &key
}

func DeriveGhostPrivateKey(R, a, b *Key, outputIndex uint64) *Key {
	x := HashScalar(KeyMultPubPriv(R, a), outputIndex)
	y, err := edwards25519.NewScalar().SetCanonicalBytes(b[:])
	if err != nil {
		panic(b.String())
	}
	t := edwards25519.NewScalar().Add(x, y)
	var key Key
	copy(key[:], t.Bytes())
	return &key
}

func ViewGhostOutputKey(P, a, R *Key, outputIndex uint64) *Key {
	x := HashScalar(KeyMultPubPriv(R, a), outputIndex)
	p1, err := edwards25519.NewIdentityPoint().SetBytes(P[:])
	if err != nil {
		panic(P.String())
	}
	p2 := edwards25519.NewIdentityPoint().ScalarBaseMult(x)
	p4 := edwards25519.NewIdentityPoint().Subtract(p1, p2)
	var key Key
	copy(key[:], p4.Bytes())
	return &key
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
