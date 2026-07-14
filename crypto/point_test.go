package crypto

import (
	"bytes"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestDecodePointCacheReturnsIndependentPoints(t *testing.T) {
	key := NewKeyFromSeed(testSeed(90)).Public()

	first, err := decodePoint(key[:])
	require.NoError(t, err)
	second, err := decodePoint(key[:])
	require.NoError(t, err)
	require.NotSame(t, first, second)

	first.Negate(first)
	third, err := decodePoint(key[:])
	require.NoError(t, err)
	require.Equal(t, key[:], third.Bytes())
}

func TestDecodedPointCacheEvictsOldestEntry(t *testing.T) {
	var shard decodedPointShard
	point := edwards25519.NewGeneratorPoint()
	var first, last [32]byte
	for i := range decodedPointCacheEntriesShard + 1 {
		var key [32]byte
		key[0] = byte(i >> 8)
		key[1] = byte(i)
		if i == 0 {
			first = key
		}
		last = key
		shard.store(key, point)
	}

	require.Nil(t, shard.load(first))
	require.NotNil(t, shard.load(last))
	require.Len(t, shard.points, decodedPointCacheEntriesShard)
}

func BenchmarkDecodePointCached(b *testing.B) {
	key := NewKeyFromSeed(testSeed(91)).Public()
	_, err := decodePoint(key[:])
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		point, err := decodePoint(key[:])
		if err != nil || point.Equal(edwards25519.NewIdentityPoint()) == 1 {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodePointUncached(b *testing.B) {
	key := NewKeyFromSeed(testSeed(92)).Public()

	b.ResetTimer()
	for range b.N {
		point, err := edwards25519.NewIdentityPoint().SetBytes(key[:])
		if err != nil || !isPrimeOrderPoint(point) || !bytes.Equal(key[:], point.Bytes()) {
			b.Fatal(err)
		}
	}
}
