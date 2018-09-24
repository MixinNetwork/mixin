package storage

import (
	"errors"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

var (
	ErrorNotFound = errors.New("value not found")
)

type Store interface {
	StateGet(key string, val interface{}) error
	StateSet(key string, val interface{}) error

	SnapshotsLoadGenesis([]*common.Snapshot) error
	SnapshotsGetUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	SnapshotsGetKey(key crypto.Key) (bool, error)
	SnapshotsListSince(offset, count uint64) ([]*common.SnapshotWithHash, error)
	SnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
