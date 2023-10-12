package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) checkTxInStorage(id crypto.Hash) (*common.VersionedTransaction, error) {
	tx, _, err := node.persistStore.ReadTransaction(id)
	if err != nil || tx != nil {
		return tx, err
	}

	return node.persistStore.CacheGetTransaction(id)
}
