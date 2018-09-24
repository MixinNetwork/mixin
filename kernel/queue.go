package kernel

import (
	"log"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store.SnapshotsGetUTXO, store.SnapshotsGetKey)
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
			log.Println(k, tx)
			s, err := node.buildSnapshot(&tx)
			if err != nil {
				return err
			}
			err = node.feedMempool(s)
			if err != nil {
				return err
			}
			offset = k + 1
			return nil
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (node *Node) buildSnapshot(tx *common.SignedTransaction) (*common.Snapshot, error) {
	return &common.Snapshot{
		Transaction: tx,
	}, nil
}
