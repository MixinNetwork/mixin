package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/mobilecoin"
	"github.com/stretchr/testify/assert"
)

func TestAsset(t *testing.T) {
	assert := assert.New(t)

	mob := &Asset{
		ChainId:  mobilecoin.MobileCoinChainId,
		AssetKey: "eea900a8-b327-488c-8d8d-1428702fe240",
	}
	eusd := &Asset{
		ChainId:  mobilecoin.MobileCoinChainId,
		AssetKey: "MCIP0025:1",
	}

	assert.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), mob.FeeAssetId())
	assert.Equal(crypto.NewHash([]byte("659c407a-0489-30bf-9e6f-84ef25c971c9")), eusd.FeeAssetId())
}
