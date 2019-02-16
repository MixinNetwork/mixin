package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store)
	if err != nil {
		return "", err
	}
	err = store.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	snap := &common.Snapshot{
		Transaction: tx.PayloadHash(),
	}
	return tx.PayloadHash().String(), store.QueueAppendSnapshot(snap)
}

func (node *Node) ConsumeQueue() error {
	var offset = uint64(0)
	for {
		err := node.store.QueuePollSnapshots(offset, func(k uint64, v []byte) error {
			var tx common.SignedTransaction
			err := msgpack.Unmarshal(v, &tx)
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
