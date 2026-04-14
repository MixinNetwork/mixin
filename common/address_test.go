package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVanishAddress(t *testing.T) {
	require := require.New(t)
	require.Panics(func() {
		_ = NewAddressFromSeed(bytes.Repeat([]byte{0}, 64))
	})
	require.NotPanics(func() {
		_ = NewAddressFromSeed(bytes.Repeat([]byte{1}, 64))
	})
	sa := "XINSwYaJPnKiwBWqXm4i3e3My9GKguReMRyB1sRSexeHcQ7V66RWsicAiR2dokcQ5kiJsfY5QbEjTcqRQRCxkEyENBaz4AeB"
	a := NewAddressFromSeed(bytes.Repeat([]byte{1}, 64))
	require.Equal(sa, a.String())
	require.NotPanics(func() {
		_, _ = NewAddressFromString(sa)
	})
	a, err := NewAddressFromString(sa)
	require.Nil(err)
	require.Equal(sa, a.String())
}

func TestAddress(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	addr := "XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8"

	_, err := NewAddressFromString(addr[:95] + "7")
	require.NotNil(err)

	a := NewAddressFromSeed(seed)
	require.Equal(addr, a.String())
	require.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", a.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	require.Equal("8665767180c62fa337b2ff051e0387af66f6feb46acacb82884b062f1fd5ed0b", a.PrivateViewKey.String())
	require.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", a.PrivateSpendKey.String())
	require.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", a.Hash().String())

	j, err := a.MarshalJSON()
	require.Nil(err)
	require.Equal("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8\"", string(j))
	err = a.UnmarshalJSON([]byte("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZpEVMtrR6PjtmgtNAH6jrg8dTUQFb9waqqw9euU7Ea8AC6DEu8\""))
	require.Nil(err)
	require.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", a.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", a.PrivateViewKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", a.PrivateSpendKey.String())
	require.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", a.Hash().String())

	b, err := NewAddressFromString(addr)
	require.Nil(err)
	require.Equal(addr, b.String())
	require.Equal("af8f69545b784e71de5e0a0261cb107aea99e9d7fe0df35537899cd9f05ea644", b.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", b.PublicSpendKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", b.PrivateViewKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", b.PrivateSpendKey.String())
	require.Equal("013ada6acca01c3ba1fce30afa922a029bb224d4ab158127428b9e85c7175c32", b.Hash().String())

	z := NewAddressFromSeed(bytes.Repeat([]byte{1}, 64))
	require.Equal("XINSwYaJPnKiwBWqXm4i3e3My9GKguReMRyB1sRSexeHcQ7V66RWsicAiR2dokcQ5kiJsfY5QbEjTcqRQRCxkEyENBaz4AeB", z.String())
	err = a.UnmarshalJSON([]byte("\"\""))
	require.NotNil(err)
}
