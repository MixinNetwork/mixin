package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	"filippo.io/edwards25519"
)

// CosiCommitment is a pair of Schnorr nonce commitments (R1, R2) = (r1·G, r2·G).
// The effective nonce of a signing session is R1 + b·R2, where the coefficient
// b is bound to the whole session transcript. An adversary therefore cannot
// know the effective nonce when the commitment is published, which blocks the
// concurrent-session (ROS/Wagner) forgery attacks on two-round multi-signatures
// that a single commitment per session is exposed to.
//
// See MuSig2: Simple Two-Round Schnorr Multi-Signatures,
// https://eprint.iacr.org/2020/1261
type CosiCommitment struct {
	rPub1, rPub2 Key
}

// NewCosiCommitment builds a commitment pair from its two components.
func NewCosiCommitment(rPub1, rPub2 Key) CosiCommitment {
	if !rPub1.CheckKey() {
		panic(rPub1.String())
	}
	if !rPub2.CheckKey() {
		panic(rPub2.String())
	}
	return CosiCommitment{rPub1: rPub1, rPub2: rPub2}
}

// points decodes both commitment components to curve points. The compressed
// keys remain the canonical form: they keep the commitment comparable and
// usable as a map key, while decoding is cached by decodePoint.
func (c *CosiCommitment) points() (*edwards25519.Point, *edwards25519.Point, error) {
	p1, err := decodePoint(c.rPub1[:])
	if err != nil {
		return nil, nil, err
	}
	p2, err := decodePoint(c.rPub2[:])
	if err != nil {
		return nil, nil, err
	}
	return p1, p2, nil
}

// Bytes returns the 64-byte wire encoding R1 ‖ R2 of the commitment pair.
func (c CosiCommitment) Bytes() []byte {
	var b [64]byte
	copy(b[:32], c.rPub1[:])
	copy(b[32:], c.rPub2[:])
	return b[:]
}

// CosiCommitmentFromBytes decodes a 64-byte wire encoding R1 ‖ R2.
func CosiCommitmentFromBytes(data []byte) (*CosiCommitment, error) {
	if len(data) != 64 {
		return nil, fmt.Errorf("invalid cosi commitment size %d", len(data))
	}
	var c CosiCommitment
	copy(c.rPub1[:], data[:32])
	copy(c.rPub2[:], data[32:])
	if !c.rPub1.CheckKey() {
		return nil, fmt.Errorf("invalid key R1 %s", c.rPub1)
	}
	if !c.rPub2.CheckKey() {
		return nil, fmt.Errorf("invalid key R2 %s", c.rPub2)
	}
	return &c, nil
}

type CosiSignature struct {
	Signature   Signature
	Mask        uint64
	commitments map[int]*CosiCommitment
	randoms     *CosiCommitment
}

// cosiNonceCoefficientDomain separates the nonce-coefficient hash from the
// challenge hash and every other SHA-512 usage in the protocol.
const cosiNonceCoefficientDomain = "MIXIN_COSI_NONCE_COEF_V1"

func cosiNonceCoefficient(R1, R2, A *Key, message Hash) (*edwards25519.Scalar, error) {
	var digest [64]byte
	h := sha512.New()
	h.Write([]byte(cosiNonceCoefficientDomain))
	h.Write(R1[:])
	h.Write(R2[:])
	h.Write(A[:])
	h.Write(message[:])
	h.Sum(digest[:0])
	return edwards25519.NewScalar().SetUniformBytes(digest[:])
}

func CosiCommitNonce(randReader io.Reader) *CosiNonce {
	return newCosiNonce(cosiCommit(randReader), cosiCommit(randReader))
}

func cosiCommit(randReader io.Reader) *Key {
	var messageDigest [64]byte
	n, err := randReader.Read(messageDigest[:])
	if err != nil {
		panic(err)
	}
	if n != len(messageDigest) {
		panic(fmt.Errorf("rand read %d %d", len(messageDigest), n))
	}
	r := NewKeyFromSeed(messageDigest[:])
	return &r
}

func CosiAggregateCommitment(randoms map[int]*CosiCommitment, publics []*Key, message Hash) (*CosiSignature, error) {
	if len(randoms) == 0 {
		return nil, fmt.Errorf("empty cosi commitments")
	}
	cosi := &CosiSignature{commitments: make(map[int]*CosiCommitment)}
	P1 := edwards25519.NewIdentityPoint()
	P2 := edwards25519.NewIdentityPoint()
	for i, R := range randoms {
		if R == nil {
			return nil, fmt.Errorf("nil cosi commitment %d", i)
		}
		p1, p2, err := R.points()
		if err != nil {
			return nil, err
		}
		P1 = P1.Add(P1, p1)
		P2 = P2.Add(P2, p2)
		err = cosi.mark(i)
		if err != nil {
			return nil, err
		}
		cosi.commitments[i] = R
	}
	aggregate := &CosiCommitment{}
	copy(aggregate.rPub1[:], P1.Bytes())
	copy(aggregate.rPub2[:], P2.Bytes())
	cosi.randoms = aggregate

	A, err := cosi.aggregatePublicKey(publics)
	if err != nil {
		return nil, err
	}
	b, err := cosiNonceCoefficient(&aggregate.rPub1, &aggregate.rPub2, A, message)
	if err != nil {
		return nil, err
	}
	effective := edwards25519.NewIdentityPoint().ScalarMult(b, P2)
	effective = effective.Add(effective, P1)
	copy(cosi.Signature[:32], effective.Bytes())
	return cosi, nil
}

// Randoms returns the aggregate nonce commitment pair of the session, or nil
// for a signature outside the interactive protocol, e.g. one loaded from a
// finalized snapshot.
func (c *CosiSignature) Randoms() *CosiCommitment {
	return c.randoms
}

// SetRandoms attaches the aggregate nonce commitment pair to a signature
// reconstructed from the wire, e.g. a challenge parsed from a peer message.
func (c *CosiSignature) SetRandoms(randoms *CosiCommitment) {
	c.randoms = randoms
}

// nonceCoefficient derives the session nonce coefficient b. It is bound to
// both aggregate nonce components, the aggregate key, and the message, so it
// is unknowable when the commitments are published.
func (c *CosiSignature) nonceCoefficient(publics []*Key, message Hash) (*edwards25519.Scalar, error) {
	if c.randoms == nil {
		return nil, fmt.Errorf("missing cosi randoms")
	}
	A, err := c.aggregatePublicKey(publics)
	if err != nil {
		return nil, err
	}
	return cosiNonceCoefficient(&c.randoms.rPub1, &c.randoms.rPub2, A, message)
}

// ensureEffectiveRandom derives R_eff = R1 + b·R2 from the aggregate randoms.
// A signature without an R yet, e.g. a challenge reconstructed from the wire,
// gets the derived value. A signature with an R set, e.g. one embedded by the
// leader, must match the derived value.
func (c *CosiSignature) ensureEffectiveRandom(publics []*Key, message Hash) (*edwards25519.Scalar, error) {
	if c.randoms == nil {
		return nil, fmt.Errorf("missing cosi randoms")
	}
	b, err := c.nonceCoefficient(publics, message)
	if err != nil {
		return nil, err
	}
	rPub1, rPub2, err := c.randoms.points()
	if err != nil {
		return nil, err
	}
	effective := edwards25519.NewIdentityPoint().ScalarMult(b, rPub2)
	effective = effective.Add(effective, rPub1)
	var eff Key
	copy(eff[:], effective.Bytes())

	var zero [32]byte
	if bytes.Equal(c.Signature[:32], zero[:]) {
		copy(c.Signature[:32], eff[:])
		return b, nil
	}
	if !bytes.Equal(c.Signature[:32], eff[:]) {
		return nil, fmt.Errorf("cosi effective random mismatch")
	}
	return b, nil
}

// signingScalars validates the effective random before deriving the challenge.
// The value receiver keeps reconstruction local so concurrent signing calls
// can share a wire signature without mutating its missing effective random.
func (c CosiSignature) signingScalars(publics []*Key, message Hash) (*edwards25519.Scalar, *edwards25519.Scalar, error) {
	coefficient, err := c.ensureEffectiveRandom(publics, message)
	if err != nil {
		return nil, nil, err
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return nil, nil, err
	}
	return coefficient, challenge, nil
}

// VerifyAnnouncementResponse verifies that the response embedded in the
// signature's S part is a valid response by the holder of public for the
// session defined by the aggregate randoms, against that signer's announced
// commitment pair. The signature is not mutated.
func (c CosiSignature) VerifyAnnouncementResponse(public *Key, announcement *CosiCommitment, publics []*Key, message Hash) error {
	coefficient, challenge, err := c.signingScalars(publics, message)
	if err != nil {
		return err
	}
	var response [32]byte
	copy(response[:], c.Signature[32:])
	if !verifyCosiPairResponse(public, announcement, &response, coefficient, challenge) {
		return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(response[:]))
	}
	return nil
}

func (c *CosiSignature) AggregateResponse(publics []*Key, responses map[int]*[32]byte, message Hash, strict bool) error {
	S := edwards25519.NewScalar()
	var keys []*Key
	for _, i := range c.Keys() {
		if i >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		if responses[i] == nil {
			return fmt.Errorf("invalid cosi signature responses with missing key %d", i)
		}
		keys = append(keys, publics[i])
	}
	if len(keys) != len(responses) {
		return fmt.Errorf("invalid cosi signature responses count %d/%d", len(keys), len(responses))
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return err
	}
	var coefficient *edwards25519.Scalar
	if strict {
		coefficient, err = c.nonceCoefficient(publics, message)
		if err != nil {
			return err
		}
	}
	for i, s := range responses {
		if c.commitments[i] == nil {
			return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(s[:]))
		}
		if strict && !verifyCosiPairResponse(publics[i], c.commitments[i], s, coefficient, challenge) {
			return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(s[:]))
		}

		si, err := edwards25519.NewScalar().SetCanonicalBytes(s[:])
		if err != nil {
			return err
		}
		S = S.Add(S, si)
	}
	copy(c.Signature[32:], S.Bytes())
	return nil
}

func (c *CosiSignature) Challenge(publics []*Key, message Hash) (*edwards25519.Scalar, error) {
	var hramDigest [64]byte
	R := c.Signature[:32]
	A, err := c.aggregatePublicKey(publics)
	if err != nil {
		return nil, err
	}
	h := sha512.New()
	h.Write(R)
	h.Write(A[:])
	h.Write(message[:])
	h.Sum(hramDigest[:0])
	s, err := edwards25519.NewScalar().SetUniformBytes(hramDigest[:])
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (c *CosiSignature) Response(privateKey *Key, random1, random2 *Key, publics []*Key, message Hash) (*[32]byte, error) {
	coefficient, challenge, err := c.signingScalars(publics, message)
	if err != nil {
		return nil, err
	}
	return cosiResponse(privateKey, random1, random2, coefficient, challenge), nil
}

func cosiResponse(privateKey, random1, random2 *Key, coefficient, challenge *edwards25519.Scalar) *[32]byte {
	y, err := edwards25519.NewScalar().SetCanonicalBytes(privateKey[:])
	if err != nil {
		panic(privateKey.String()[:8])
	}
	z1, err := edwards25519.NewScalar().SetCanonicalBytes(random1[:])
	if err != nil {
		panic(random1.String())
	}
	z2, err := edwards25519.NewScalar().SetCanonicalBytes(random2[:])
	if err != nil {
		panic(random2.String())
	}
	var s [32]byte
	// s = r1 + b·r2 + c·x
	si := edwards25519.NewScalar().MultiplyAdd(coefficient, z2, z1)
	si = edwards25519.NewScalar().MultiplyAdd(challenge, y, si)
	copy(s[:], si.Bytes())
	return &s
}

// verifyCosiPairResponse verifies a single signer's response s against its
// nonce commitment pair: s·G == R1 + b·R2 + c·X.
func verifyCosiPairResponse(public *Key, commitment *CosiCommitment, s *[32]byte, coefficient, challenge *edwards25519.Scalar) bool {
	if public == nil || commitment == nil || s == nil || coefficient == nil || challenge == nil {
		return false
	}
	A, err := decodePoint(public[:])
	if err != nil {
		return false
	}
	R1, R2, err := commitment.points()
	if err != nil {
		return false
	}
	si, err := edwards25519.NewScalar().SetCanonicalBytes(s[:])
	if err != nil {
		return false
	}
	lhs := edwards25519.NewIdentityPoint().ScalarBaseMult(si)
	rhs := edwards25519.NewIdentityPoint().ScalarMult(coefficient, R2)
	rhs = rhs.Add(rhs, R1)
	rhs = rhs.Add(rhs, edwards25519.NewIdentityPoint().ScalarMult(challenge, A))
	return lhs.Equal(rhs) == 1
}

func (c *CosiSignature) VerifyResponse(publics []*Key, signer int, s *[32]byte, message Hash) error {
	if s == nil {
		return fmt.Errorf("nil cosi response")
	}
	var a *Key
	var R *CosiCommitment
	for _, k := range c.Keys() {
		if k >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", k, len(publics))
		}
		if k == signer {
			a = publics[k]
			R = c.commitments[k]
		}
	}
	if R == nil {
		return fmt.Errorf("invalid cosi signature mask index %d", signer)
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return err
	}
	coefficient, err := c.nonceCoefficient(publics, message)
	if err != nil {
		return err
	}
	if !verifyCosiPairResponse(a, R, s, coefficient, challenge) {
		return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(s[:]))
	}
	return nil
}

func (c *CosiSignature) mark(i int) error {
	if i >= 64 || i < 0 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.Mask ^= (1 << uint64(i))
	return nil
}

func (c *CosiSignature) Keys() []int {
	keys := make([]int, 0)
	for i := range uint64(64) {
		mask := uint64(1) << i
		if c.Mask&mask == mask {
			keys = append(keys, int(i))
		}
	}
	return keys
}

func (c *CosiSignature) aggregatePublicKey(publics []*Key) (*Key, error) {
	return aggregatePublicKey(publics, c.Keys())
}

func (c *CosiSignature) ThresholdVerify(threshold int) bool {
	return len(c.Keys()) >= threshold
}

func (c *CosiSignature) FullVerify(publics []*Key, threshold int, message Hash) error {
	if threshold <= 0 {
		return fmt.Errorf("invalid cosi threshold %d", threshold)
	}
	if !c.ThresholdVerify(threshold) {
		return fmt.Errorf("cosi.FullVerify publics %d threshold %d keys %d", len(publics), threshold, len(c.Keys()))
	}
	A, err := c.aggregatePublicKey(publics)
	if err != nil {
		return fmt.Errorf("cosi.FullVerify aggregatePublicKey %v", err)
	}
	if !A.Verify(message, c.Signature) {
		return fmt.Errorf("cosi.FullVerify signature verify failed")
	}
	return nil
}

func (c CosiSignature) String() string {
	return c.Signature.String() + fmt.Sprintf("%016x", c.Mask)
}

func (c CosiSignature) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(c.String())), nil
}

func (c *CosiSignature) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(c.Signature)+8 {
		return fmt.Errorf("invalid signature length %d", len(data))
	}
	copy(c.Signature[:], data)
	c.Mask, err = strconv.ParseUint(unquoted[len(c.Signature)*2:], 16, 64)
	if err != nil {
		return fmt.Errorf("invalid mask data %x", data)
	}
	return nil
}
