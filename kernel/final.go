package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) checkTxInStorage(id crypto.Hash) (*common.VersionedTransaction, string, error) {
	tx, snap, err := node.persistStore.ReadTransaction(id)
	if err != nil || tx != nil {
		return tx, snap, err
	}

	tx, err = node.persistStore.CacheGetTransaction(id)
	return tx, "", err
}
