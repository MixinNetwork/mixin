package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type RoundSpace struct {
	NodeId   crypto.Hash
	Batch    uint64
	Round    uint64
	Duration uint64
}
