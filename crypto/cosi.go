package crypto

import (
	"fmt"
	"io"
)

type CosiSignature struct {
	Signatures []Signature `json:"signatures"`
	Mask       uint64      `json:"mask"`
	sigMask    uint64      `json:"-" msgpack:"-"`
}

func CosiCommit(randReader io.Reader) (PrivateKey, error) {
	var messageDigest [64]byte
	n, err := randReader.Read(messageDigest[:])
	if err != nil {
		return nil, err
	}
	if n != len(messageDigest) {
		return nil, fmt.Errorf("rand read %d %d", len(messageDigest), n)
	}
	return keyFactory.NewPrivateKeyFromSeed(messageDigest[:])
}

func CosiCommitPanic(randReader io.Reader) PrivateKey {
	key, err := CosiCommit(randReader)
	if err != nil {
		panic(err)
	}
	return key
}

func CosiAggregateCommitment(commitents map[int]PublicKey) (*CosiSignature, error) {
	cosi := CosiSignature{}
	for i := range commitents {
		err := cosi.Mark(i)
		if err != nil {
			return nil, err
		}
	}
	if err := keyFactory.CosiLoadCommitents(&cosi, commitents); err != nil {
		return nil, err
	}
	cosi.sigMask = cosi.Mask
	return &cosi, nil
}

func (c *CosiSignature) Dumps() ([]byte, error) {
	return keyFactory.CosiDumps(c)
}

func (c *CosiSignature) Loads(data []byte) (rest []byte, err error) {
	return keyFactory.CosiLoads(c, data)
}

func (c *CosiSignature) Mark(i int) error {
	if i >= 64 || i < 0 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.Mask ^= (1 << uint64(i))
	return nil
}

func (c *CosiSignature) KeyIndex(node int) int {
	for i, k := range c.Keys() {
		if k == node {
			return i
		}
	}
	return -1
}

func (c *CosiSignature) Keys() []int {
	keys := make([]int, 0)
	for i := uint64(0); i < 64; i++ {
		mask := uint64(1) << i
		if c.Mask&mask == mask {
			keys = append(keys, int(i))
		}
	}
	return keys
}

func (c *CosiSignature) MarkSignature(i int) error {
	if i >= 64 || i < 0 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.sigMask ^= (1 << uint64(i))
	return nil
}

func (c *CosiSignature) SignatureAggregated(i int) bool {
	if i >= 64 || i < 0 {
		return false
	}
	return c.sigMask^(1<<uint64(i)) != c.sigMask
}

func (c *CosiSignature) SignatureMasks() uint64 {
	return c.sigMask
}

func (c *CosiSignature) publicKeys(allPublics []PublicKey) (map[int]PublicKey, error) {
	var keys = make(map[int]PublicKey)
	for _, i := range c.Keys() {
		if i >= len(allPublics) {
			return nil, fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(allPublics))
		}
		keys[i] = allPublics[i]
	}
	return keys, nil
}

func (c *CosiSignature) Challenge(allPublics []PublicKey, message []byte) ([32]byte, error) {
	pubs, err := c.publicKeys(allPublics)
	if err != nil {
		return [32]byte{}, err
	}
	return keyFactory.CosiChallenge(c, pubs, message)
}

func (c *CosiSignature) AggregateSignature(node int, sig Signature) error {
	index := c.KeyIndex(node)
	if index < 0 {
		return fmt.Errorf("invalid node %d", node)
	}

	// already added
	if c.sigMask^(1<<uint64(index)) == c.sigMask {
		return nil
	}
	return keyFactory.CosiAggregateSignature(c, index, sig)
}

func (c *CosiSignature) ThresholdVerify(threshold int) bool {
	return len(c.Keys()) >= threshold
}

func (c *CosiSignature) FullVerify(publics []PublicKey, threshold int, message []byte) bool {
	if c.sigMask != 0 {
		return false
	}
	if !c.ThresholdVerify(threshold) {
		return false
	}
	pubs, err := c.publicKeys(publics)
	if err != nil {
		return false
	}
	return keyFactory.CosiFullVerify(pubs, message, *c)
}

func (c *CosiSignature) DumpSignatureResponse(sig Signature) []byte {
	return keyFactory.DumpSignatureResponse(sig)
}
