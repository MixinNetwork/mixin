package crypto

import (
	"encoding/hex"
	"fmt"
)

type CosiSignature struct {
	Signatures    []Signature `json:"signatures"`
	Mask          uint64      `json:"mask"`
	SignatureMask uint64      `json:"-" msgpack:"-"`
}

func CosiAggregateCommitments(commitments map[int]*Commitment) (*CosiSignature, error) {
	cosi := CosiSignature{}
	for i := range commitments {
		err := cosi.Mark(i)
		if err != nil {
			return nil, err
		}
	}
	if err := keyFactory.CosiAggregateCommitments(&cosi, commitments); err != nil {
		return nil, err
	}
	cosi.SignatureMask = cosi.Mask
	return &cosi, nil
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

func (c *CosiSignature) SignatureAggregated(i int) bool {
	if i >= 64 || i < 0 {
		return false
	}
	mask := uint64(1 << i)
	return (c.Mask&mask) == mask && (c.SignatureMask&mask) == 0
}

func (c *CosiSignature) MarkSignature(i int) error {
	if i >= 64 || i < 0 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.SignatureMask ^= (1 << uint64(i))
	return nil
}

func (c *CosiSignature) filterPublicKeys(allPublics []PublicKey) (map[int]PublicKey, error) {
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
	pubs, err := c.filterPublicKeys(allPublics)
	if err != nil {
		return [32]byte{}, err
	}
	return keyFactory.CosiChallenge(c, pubs, message)
}

func (c *CosiSignature) AggregateSignature(node int, sig *Signature) error {
	index := c.KeyIndex(node)
	if index < 0 {
		return fmt.Errorf("invalid node %d", node)
	}

	// already added
	if c.SignatureAggregated(node) {
		return nil
	}
	if err := keyFactory.CosiAggregateSignature(c, index, sig); err != nil {
		return err
	}
	return c.MarkSignature(node)
}

func (c *CosiSignature) ThresholdVerify(threshold int) bool {
	return len(c.Keys()) >= threshold
}

func (c *CosiSignature) FullVerify(publics []PublicKey, threshold int, message []byte) bool {
	if c.SignatureMask != 0 {
		return false
	}
	if !c.ThresholdVerify(threshold) {
		return false
	}
	pubs, err := c.filterPublicKeys(publics)
	if err != nil {
		return false
	}
	return keyFactory.CosiFullVerify(pubs, message, c)
}

func (c CosiSignature) String() string {
	return hex.EncodeToString(c.Dumps())
}

func (c CosiSignature) Dumps() []byte {
	return keyFactory.CosiDumps(&c)
}

func (c *CosiSignature) Loads(data []byte) (rest []byte, err error) {
	return keyFactory.CosiLoads(c, data)
}

func (c *CosiSignature) DumpSignatureResponse(sig *Signature) *Response {
	return keyFactory.DumpSignatureResponse(sig)
}

func (c *CosiSignature) LoadResponseSignature(commitment *Commitment, response *Response) *Signature {
	return keyFactory.LoadResponseSignature(c, commitment, response)
}
