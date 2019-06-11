package crypto

import "fmt"

type CosiSignature struct {
	Signature Signature
	Mask      uint64
}

func (c *CosiSignature) Mark(i int) error {
	if i >= 64 {
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

func (c *CosiSignature) AggregatePublicKey(publics []Key) (*Key, error) {
	var key *Key
	for i := range c.Keys() {
		if i >= len(publics) {
			return nil, fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		k := publics[i]
		if key == nil {
			key = &k
		} else {
			key = KeyAdd(key, &k)
		}
	}
	return key, nil
}
