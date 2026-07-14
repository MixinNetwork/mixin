package crypto

import (
	"bytes"
	"fmt"
	"sync"

	"filippo.io/edwards25519"
)

const (
	decodedPointCacheShards         = 64
	decodedPointCacheEntriesMaximum = 1024 * 32
)

type decodedPointShard struct {
	sync.RWMutex
	points map[[32]byte]*edwards25519.Point
	order  [decodedPointCacheEntriesMaximum][32]byte
	next   int
}

var decodedPoints [decodedPointCacheShards]decodedPointShard

var invEightScalar = func() *edwards25519.Scalar {
	var eightBytes [32]byte
	eightBytes[0] = 8
	eight, err := edwards25519.NewScalar().SetCanonicalBytes(eightBytes[:])
	if err != nil {
		panic(err)
	}
	return edwards25519.NewScalar().Invert(eight)
}()

func decodePoint(src []byte) (*edwards25519.Point, error) {
	var key [32]byte
	if len(src) == len(key) {
		copy(key[:], src)
		shard := &decodedPoints[key[0]&(decodedPointCacheShards-1)]
		if cached := shard.load(key); cached != nil {
			return cached, nil
		}
	}

	p, err := edwards25519.NewIdentityPoint().SetBytes(src)
	if err != nil {
		return nil, err
	}
	if !isPrimeOrderPoint(p) {
		return nil, fmt.Errorf("invalid point subgroup")
	}
	if !bytes.Equal(src, p.Bytes()) {
		return nil, fmt.Errorf("invalid point encoding")
	}

	decodedPoints[key[0]&(decodedPointCacheShards-1)].store(key, p)
	return p, nil
}

func (shard *decodedPointShard) load(key [32]byte) *edwards25519.Point {
	shard.RLock()
	defer shard.RUnlock()

	point := shard.points[key]
	if point == nil {
		return nil
	}
	return edwards25519.NewIdentityPoint().Set(point)
}

func (shard *decodedPointShard) store(key [32]byte, point *edwards25519.Point) {
	shard.Lock()
	defer shard.Unlock()

	if shard.points == nil {
		shard.points = make(map[[32]byte]*edwards25519.Point)
	}
	if shard.points[key] != nil {
		return
	}
	if len(shard.points) == decodedPointCacheEntriesMaximum {
		delete(shard.points, shard.order[shard.next])
	}
	shard.points[key] = edwards25519.NewIdentityPoint().Set(point)
	shard.order[shard.next] = key
	shard.next = (shard.next + 1) % decodedPointCacheEntriesMaximum
}

func isPrimeOrderPoint(p *edwards25519.Point) bool {
	if p.Equal(edwards25519.NewIdentityPoint()) == 1 {
		return false
	}
	cleared := edwards25519.NewIdentityPoint().MultByCofactor(p)
	restored := edwards25519.NewIdentityPoint().ScalarMult(invEightScalar, cleared)
	return restored.Equal(p) == 1
}
