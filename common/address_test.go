package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	addr := "XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZLDU9N4dj4sz7YELpTKFk1UuDXPLTuikeQHQYzS87aQkckFNRT"

	_, err := NewAddressFromString(addr[:95] + "7")
	require.NotNil(err)

	a := NewAddressFromSeed(seed)
	require.Equal(addr, a.String())
	require.Equal("6ff15bbe96e92bf0b3609d9582926a24a6f394bfc2dffb283d9623a960b4569c", a.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	require.Equal("2a3d8a83a21f6eb6e6d05b93e9d94a09b1e37dc7d0cb195249ae83bc55cff909", a.PrivateViewKey.String())
	require.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", a.PrivateSpendKey.String())
	require.Equal("b8433ef0c6689929a965b4a49226fe7d9253ca2a4890e31eac40c658956e559d", a.Hash().String())

	j, err := a.MarshalJSON()
	require.Nil(err)
	require.Equal("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZLDU9N4dj4sz7YELpTKFk1UuDXPLTuikeQHQYzS87aQkckFNRT\"", string(j))
	err = a.UnmarshalJSON([]byte("\"XIN8AJMgQUD11jZYN9ggbQDqkmozrha3zPEZxEkKxVFBufZLDU9N4dj4sz7YELpTKFk1UuDXPLTuikeQHQYzS87aQkckFNRT\""))
	require.Nil(err)
	require.Equal("6ff15bbe96e92bf0b3609d9582926a24a6f394bfc2dffb283d9623a960b4569c", a.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", a.PublicSpendKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", a.PrivateViewKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", a.PrivateSpendKey.String())
	require.Equal("b8433ef0c6689929a965b4a49226fe7d9253ca2a4890e31eac40c658956e559d", a.Hash().String())

	b, err := NewAddressFromString(addr)
	require.Nil(err)
	require.Equal(addr, b.String())
	require.Equal("6ff15bbe96e92bf0b3609d9582926a24a6f394bfc2dffb283d9623a960b4569c", b.PublicViewKey.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", b.PublicSpendKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", b.PrivateViewKey.String())
	require.Equal("0000000000000000000000000000000000000000000000000000000000000000", b.PrivateSpendKey.String())
	require.Equal("b8433ef0c6689929a965b4a49226fe7d9253ca2a4890e31eac40c658956e559d", b.Hash().String())

	z := NewAddressFromSeed(make([]byte, 64))
	require.Equal("XIN8b7CsqwqaBP7576hvWzo7uDgbU9TB5KGU4jdgYpQTi369bdVnXkAC6dJ1TBspvMPuuHcEuEF6m1pu3QEpw3ojAfPyZjK", z.String())
}
