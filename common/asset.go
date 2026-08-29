package common

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	XINAssetId          = crypto.Sha256Hash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
	BitcoinAssetId      = crypto.Sha256Hash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa"))
	EthereumAssetId     = crypto.Sha256Hash([]byte("43d61dcd-e413-450d-80b8-101d5e903357"))
	BOXAssetId          = crypto.Sha256Hash([]byte("f5ef6b5d-cc5a-3d90-b2c0-a2fd386e7a3c"))
	MOBAssetId          = crypto.Sha256Hash([]byte("eea900a8-b327-488c-8d8d-1428702fe240"))
	USDTEthereumAssetId = crypto.Sha256Hash([]byte("4d8c508b-91c5-375b-92b0-ee702ed2dac5"))
	USDTTRONAssetId     = crypto.Sha256Hash([]byte("b91e18ff-a9ae-3dc7-8679-e935d9a4b34b"))
	USDTBNBAssetId      = crypto.Sha256Hash([]byte("94213408-4ee7-3150-a9c4-9c5cce421c78"))
	PandoUSDAssetId     = crypto.Sha256Hash([]byte("31d2ea9c-95eb-3355-b65b-ba096853bc18"))
	USDCEthereumAssetId = crypto.Sha256Hash([]byte("9b180ab6-6abe-3dc0-a13f-04169eb34bfa"))
	USDCSolanaAssetId   = crypto.Sha256Hash([]byte("de6fa523-c596-398e-b12f-6d6980544b59"))
	EOSAssetId          = crypto.Sha256Hash([]byte("6cfe566e-4aad-470b-8c9a-2fd35b49c68d"))
	SOLAssetId          = crypto.Sha256Hash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887"))
	UNIAssetId          = crypto.Sha256Hash([]byte("a31e847e-ca87-3162-b4d1-322bc552e831"))
	DOGEAssetId         = crypto.Sha256Hash([]byte("6770a1e5-6086-44d5-b60f-545f9d9e8ffd"))
	ZECAssetId          = crypto.Sha256Hash([]byte("c996abc9-d94e-4494-b1cf-2a3fd3ac5714"))
	XRPAssetId          = crypto.Sha256Hash([]byte("23dfb5a5-5d7b-48b6-905f-3970e3176e27"))
	XMRAssetId          = crypto.Sha256Hash([]byte("05c5ac01-31f9-4a69-aa8a-ab796de1d041"))

	XINAsset = &Asset{Chain: EthereumAssetId, AssetKey: "0xa974c709cfb4566686553a20790685a47aceaa33"}
)

type Asset struct {
	Chain    crypto.Hash
	AssetKey string
}

func (a *Asset) Verify() error {
	if !a.Chain.HasValue() {
		return fmt.Errorf("invalid asset chain %v", *a)
	}
	if strings.TrimSpace(a.AssetKey) != a.AssetKey || len(a.AssetKey) == 0 {
		return fmt.Errorf("invalid asset key %v", *a)
	}
	return nil
}

func GetAssetCapacity(id crypto.Hash) Integer {
	switch id {
	case BitcoinAssetId:
		return NewIntegerFromString("2500")
	case EthereumAssetId:
		return NewIntegerFromString("5000")
	case XINAssetId:
		return NewIntegerFromString("750000")
	case BOXAssetId:
		return NewIntegerFromString("200000000")
	case MOBAssetId:
		return NewIntegerFromString("30000000")
	case USDTEthereumAssetId:
		return NewIntegerFromString("20000000")
	case USDTTRONAssetId:
		return NewIntegerFromString("25000000")
	case USDTBNBAssetId:
		return NewIntegerFromString("3000000")
	case PandoUSDAssetId:
		return NewIntegerFromString("1000000000000")
	case USDCEthereumAssetId:
		return NewIntegerFromString("3000000")
	case USDCSolanaAssetId:
		return NewIntegerFromString("3000000")
	case EOSAssetId:
		return NewIntegerFromString("3500000")
	case SOLAssetId:
		return NewIntegerFromString("60000")
	case UNIAssetId:
		return NewIntegerFromString("1100000")
	case DOGEAssetId:
		return NewIntegerFromString("25000000")
	case ZECAssetId:
		return NewIntegerFromString("5000")
	case XMRAssetId:
		return NewIntegerFromString("2000")
	case XRPAssetId:
		return NewIntegerFromString("1000000")
	default: // TODO more assets and better default value
		return NewIntegerFromString("115792089237316195423570985008687907853269984665640564039457.58400791")
	}
}
