package kernel

import (
	"log"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store.SnapshotsGetUTXO, store.SnapshotsCheckGhost)
	if err != nil {
		return "", err
	}
	return tx.Hash().String(), store.QueueAdd(tx)
}

func (node *Node) ConsumeQueue() error {
	var offset = uint64(0)
	for {
		if !node.syncrhoinized {
			time.Sleep(1 * time.Second)
			continue
		}
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

func (node *Node) buildSnapshot(tx *common.SignedTransaction) (*common.Snapshot, error) {
	snapshot := &common.Snapshot{
		NodeId:      node.IdForNetwork(),
		Transaction: tx,
		References:  []crypto.Hash{node.RoundHash, node.RoundPeer.RoundHash},
		RoundNumber: node.RoundNumber,
		Timestamp:   uint64(time.Now().UnixNano()),
	}
	common.SignSnapshot(snapshot, node.Account.PrivateSpendKey)
	return snapshot, nil
}
