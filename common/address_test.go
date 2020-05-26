// +build ed25519 !custom_alg

package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto/ed25519"
	"github.com/stretchr/testify/assert"
)

func init() {
	ed25519.Load()
}

func TestAddress(t *testing.T) {
	assert := assert.New(t)

	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	addr := "XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8"

	_, err := NewAddressFromString(addr[:95] + "7")
	assert.NotNil(err)

	a := NewAddressFromSeed(seed)
	assert.Equal(addr, a.String())
	assert.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", a.PublicViewKey.String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	assert.Equal("8665767180c62fa337b2ff051e0387af66f6feb46acacb82884b062f1fd5ed0b", a.PrivateViewKey.String())
	assert.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", a.PrivateSpendKey.String())
	assert.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", a.Hash().String())

	j, err := a.MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8\"", string(j))
	err = a.UnmarshalJSON([]byte("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8\""))
	assert.Nil(err)
	assert.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", a.PublicViewKey.String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	assert.Nil(a.PrivateViewKey)
	assert.Nil(a.PrivateSpendKey)
	assert.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", a.Hash().String())

	b, err := NewAddressFromString(addr)
	assert.Nil(err)
	assert.Equal(addr, b.String())
	assert.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", b.PublicViewKey.String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", b.PublicSpendKey.String())
	assert.Nil(b.PrivateViewKey)
	assert.Nil(b.PrivateSpendKey)
	assert.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", b.Hash().String())

	z := NewAddressFromSeed(make([]byte, 64))
	assert.Equal("XIN8b7CsqwqaBP7576hvWzo7uDgbU9TB5KGU4jdgYpQTi2qrQGpBtrW49ENQiLGNrYU45e2wwKRD7dEUPtuaJYps2jbR4dH", z.String())
}
