package common

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegerUnmarshalJSONRejectsInvalidAmounts(t *testing.T) {
	for _, data := range []string{`"invalid"`, `""`, `"-1"`, `"-0.00000001"`, `"1einvalid"`, `"1e2147483647"`, `12`, `null`} {
		t.Run(data, func(t *testing.T) {
			amount := NewInteger(42)
			require.Error(t, json.Unmarshal([]byte(data), &amount))
			require.Equal(t, "42.00000000", amount.String())
		})
	}
	amount := NewInteger(42)
	require.NoError(t, json.Unmarshal([]byte(`"1.234567899"`), &amount))
	require.Equal(t, "1.23456789", amount.String())
}

func TestIntegerScientificNotation(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"1.23456789e2", "123.45678900"},
		{"+12.3E-1", "1.23000000"},
		{"1.234567899e-3", "0.00123456"},
		{"123456789e-16", "0.00000001"},
		{"123456789e-17", "0.00000000"},
		{"1e-10000", "0.00000000"},
		{"1e10000", "1" + strings.Repeat("0", 10000) + ".00000000"},
		{"0e-10000", "0.00000000"},
		{"0e10000", "0.00000000"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var amount Integer
			data, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &amount))
			require.Equal(t, tc.want, amount.String())
		})
	}
	for _, input := range []string{
		"1e-10001", "1e10001", "0e-10001", "0e10001",
		"1e-2147483648", "0e-2147483648", "0e2147483647",
		"1e2147483639", "1e2147483647", "1e160000",
	} {
		t.Run(input, func(t *testing.T) {
			amount := NewInteger(42)
			data, err := json.Marshal(input)
			require.NoError(t, err)
			require.ErrorContains(t, json.Unmarshal(data, &amount), "MaxDecodeExponent")
			require.Equal(t, "42.00000000", amount.String())
		})
	}
}

func TestIntegerMaximumEncodedSize(t *testing.T) {
	var maximum Integer
	maximum.i.SetBytes(bytes.Repeat([]byte{0xff}, MaximumEncodingInt))
	data, err := json.Marshal(maximum)
	require.NoError(t, err)
	var parsed Integer
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Zero(t, parsed.Cmp(maximum))
	enc := NewEncoder()
	enc.WriteInteger(parsed)
	decoded, err := NewDecoder(enc.Bytes()).ReadInteger()
	require.NoError(t, err)
	require.Zero(t, decoded.Cmp(maximum))

	var overflow Integer
	overflow.i.Add(&maximum.i, big.NewInt(1))
	data, err = json.Marshal(overflow)
	require.NoError(t, err)
	require.ErrorContains(t, json.Unmarshal(data, &parsed), "maximum encoded size")
	require.Zero(t, parsed.Cmp(maximum))
}

func TestInteger(t *testing.T) {
	require := require.New(t)

	require.Equal("0.00000000", NewInteger(0).String())

	a := NewInteger(10000)
	b := NewIntegerFromString("10000")
	require.Equal(0, a.Cmp(b))

	c := a.Add(b)
	require.Equal("20000.00000000", c.String())
	j, err := c.MarshalJSON()
	require.Nil(err)
	require.Equal("\"20000.00000000\"", string(j))
	err = c.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("20000.00000000", c.String())

	require.Equal(0, b.Add(a).Cmp(c))
	require.Equal(0, c.Sub(a).Cmp(b))
	require.Equal(0, c.Sub(b).Cmp(a))

	a = NewIntegerFromString("0.000000001")
	require.Equal("0.00000000", a.String())
	a = NewIntegerFromString("10.000000001")
	require.Equal("10.00000000", a.String())
	a = NewIntegerFromString("0.00000001")
	require.Equal("0.00000001", a.String())
	a = NewIntegerFromString("10.00000001")
	require.Equal("10.00000001", a.String())
	a = NewIntegerFromString("0.1")
	require.Equal("0.10000000", a.String())

	m := NewInteger(500000)
	n := m.Div(10)
	require.Equal("50000.00000000", n.String())
	n = m.Div(1000000)
	require.Equal("0.50000000", n.String())
	n = n.Div(10000000)
	require.Equal("0.00000005", n.String())
	require.Equal(1, n.Sign())
	n = n.Mul(10).Div(10)
	require.Equal("0.00000005", n.String())
	require.Equal(1, n.Sign())
	n = n.Div(10).Mul(10)
	require.Equal("0.00000000", n.String())
	require.Equal(0, n.Sign())

	m = NewInteger(1)
	n = m.Div(3)
	require.Equal("0.33333333", n.String())
	n = n.Mul(3)
	require.Equal("0.99999999", n.String())
	n = n.Add(NewIntegerFromString("0.00000001"))
	require.Equal("1.00000000", n.String())

	m = NewInteger(8273)
	require.Equal("8273.00000000", m.String())

	m = NewIntegerFromString("0.00000192")
	require.Equal("0.00000192", m.String())

	i0 := NewIntegerFromString("0.003790547948714634")
	require.Equal("0.00379054", i0.String())
	i1 := NewIntegerFromString(i0.String())
	require.Equal("0.00379054", i1.String())
	require.Equal(i0, i1)
}
