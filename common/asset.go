package common

import "github.com/MixinNetwork/mixin/crypto"

var XINAssetId crypto.Hash

func init() {
	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
}
