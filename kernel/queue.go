package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store.SnapshotsReadUTXO, store.SnapshotsLockUTXO, store.SnapshotsCheckGhost)
	if err != nil {
		return "", err
	}
	return tx.Hash().String(), store.QueueAdd(tx)
}

func (node *Node) ConsumeQueue() error {
	var offset = uint64(0)
	for {
		err := node.store.QueuePoll(offset, func(k uint64, v []byte) error {
			var tx common.SignedTransaction
			err := msgpack.Unmarshal(v, &tx)
			if err != nil {
				return err
			}
			err = node.feedMempool(&common.Snapshot{
				NodeId:      node.IdForNetwork,
				Transaction: &tx,
			})
			if err != nil {
				return err
			}
			offset = k
			return nil
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
