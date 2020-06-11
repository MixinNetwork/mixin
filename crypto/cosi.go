package crypto

import (
	"fmt"
	"io"
)

type CosiSignature struct {
	Signatures map[int]Signature `json:"signatures"`
	Mask       uint64            `json:"mask"`
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

func CosiAggregateCommitment(commitents map[int]PublicKey) (*CosiSignature, error) {
	cosi := CosiSignature{Signatures: make(map[int]Signature, len(commitents))}
	for i := range commitents {
		err := cosi.Mark(i)
		if err != nil {
			return nil, err
		}
	}
	if err := keyFactory.CosiInitLoad(&cosi, commitents); err != nil {
		return nil, err
	}
	return &cosi, nil
}

func (c *CosiSignature) Mark(i int) error {
	if i >= 64 || i < 0 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.Mask ^= (1 << uint64(i))
	return nil
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

func (c *CosiSignature) CosiPublicKeys(allPublics []PublicKey) (map[int]PublicKey, error) {
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
	pubs, err := c.CosiPublicKeys(allPublics)
	if err != nil {
		return [32]byte{}, err
	}
	return keyFactory.CosiChallenge(c, pubs, message)
}

func (c *CosiSignature) CosiAggregateSignatures(sigs map[int]Signature) error {
	return keyFactory.CosiAggregateSignatures(c, sigs)
}

func (c *CosiSignature) ThresholdVerify(threshold int) bool {
	return len(c.Keys()) >= threshold
}

func (c *CosiSignature) FullVerify(publics []PublicKey, threshold int, message []byte) bool {
	if !c.ThresholdVerify(threshold) {
		return false
	}
	pubs, err := c.CosiPublicKeys(publics)
	if err != nil {
		return false
	}
	return keyFactory.CosiFullVerify(pubs, message, *c)
}
