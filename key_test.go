package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyNormalization(t *testing.T) {
	assert := assert.New(t)

	addr, err := NormalizeAddress(BitcoinChainId, "1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i")
	assert.Nil(err)
	assert.Equal("1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i", addr)
	addr, err = NormalizeAddress(BitcoinChainId, "1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62j")
	assert.NotNil(err)
	assert.Equal("", addr)

	addr, err = NormalizeAddress(EthereumChainId, "0x6614ecf1136B18920fe9f9C4fBa58562F73e409B")
	assert.Nil(err)
	assert.Equal("0x6614ecf1136B18920fe9f9C4fBa58562F73e409B", addr)
	addr, err = NormalizeAddress(EthereumChainId, "0x6614ecf1136B18920fe9f9C4fBa58562F73e409b")
	assert.Nil(err)
	assert.Equal("0x6614ecf1136B18920fe9f9C4fBa58562F73e409B", addr)
	addr, err = NormalizeAddress(EthereumChainId, "0x6614ecf1136B18920fe9f9C4fBa58562F73e409")
	assert.NotNil(err)
	assert.Equal("", addr)

	addr, err = NormalizeAddress(RippleChainId, "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go")
	assert.Nil(err)
	assert.Equal("rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go", addr)
	addr, err = NormalizeAddress(RippleChainId, "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Gp")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(RippleChainId, "1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i")
	assert.NotNil(err)
	assert.Equal("", addr)

	addr, err = NormalizeAddress(BitcoinCashChainId, "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd")
	assert.Nil(err)
	assert.Equal("19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd", addr)
	addr, err = NormalizeAddress(BitcoinCashChainId, "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZa")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(BitcoinCashChainId, "1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i")
	assert.Nil(err)
	assert.Equal("1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i", addr)
	addr, err = NormalizeAddress(BitcoinCashChainId, "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(BitcoinCashChainId, "LcDrhX7NCmoRj58abHjAzfNCvk7jHxARsm")
	assert.NotNil(err)
	assert.Equal("", addr)

	addr, err = NormalizeAddress(LitecoinChainId, "LcDrhX7NCmoRj58abHjAzfNCvk7jHxARsm")
	assert.Nil(err)
	assert.Equal("LcDrhX7NCmoRj58abHjAzfNCvk7jHxARsm", addr)
	addr, err = NormalizeAddress(LitecoinChainId, "LcDrhX7NCmoRj58abHjAzfNCvk7jHxARsn")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(LitecoinChainId, "1AGNa15ZQXAZUgFiqJ2i7Z2DPU2J6hW62i")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(LitecoinChainId, "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go")
	assert.NotNil(err)
	assert.Equal("", addr)
	addr, err = NormalizeAddress(LitecoinChainId, "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd")
	assert.NotNil(err)
	assert.Equal("", addr)
}

func TestLocalGenerateKey(t *testing.T) {
	assert := assert.New(t)

	public, private, err := LocalGenerateKey("invalid chain id")
	assert.NotNil(err)
	assert.Equal("unsupported chain id invalid chain id", err.Error())

	public, private, err = LocalGenerateKey(EthereumChainId)
	assert.Nil(err)
	assert.Len(public, 42)
	assert.Len(private, 64)
	addr, err := NormalizeAddress(EthereumChainId, public)
	assert.Nil(err)
	assert.Equal(public, addr)

	public, private, err = LocalGenerateKey(BitcoinChainId)
	assert.Nil(err)
	assert.Len(public, 34)
	assert.Len(private, 64)
	addr, err = NormalizeAddress(BitcoinChainId, public)
	assert.Nil(err)
	assert.Equal(public, addr)

	public, private, err = LocalGenerateKey(BitcoinCashChainId)
	assert.Nil(err)
	assert.Len(public, 34)
	assert.Len(private, 64)
	addr, err = NormalizeAddress(BitcoinCashChainId, public)
	assert.Nil(err)
	assert.Equal(public, addr)

	public, private, err = LocalGenerateKey(LitecoinChainId)
	assert.Nil(err)
	assert.Len(public, 34)
	assert.Len(private, 64)
	addr, err = NormalizeAddress(LitecoinChainId, public)
	assert.Nil(err)
	assert.Equal(public, addr)

	public, private, err = LocalGenerateKey(RippleChainId)
	assert.Nil(err)
	assert.Len(public, 34)
	assert.Len(private, 64)
	addr, err = NormalizeAddress(RippleChainId, public)
	assert.Nil(err)
	assert.Equal(public, addr)
}
