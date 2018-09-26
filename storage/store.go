package storage

import (
	"errors"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

var (
	ErrorAlreadyExist   = errors.New("key already exist")
	ErrorValidateFailed = errors.New("consensus validate failed")
)

type Store interface {
	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	SnapshotsLoadGenesis([]*common.SnapshotWithTopologicalOrder) error
	SnapshotsTopologySequence() uint64
	SnapshotsGetUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	SnapshotsCheckGhost(key crypto.Key) (bool, error)
	SnapshotsListTopologySince(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	SnapshotsListForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
	SnapshotsNodeList() ([]crypto.Hash, error)
	SnapshotsRoundMetaForNode(nodeIdWithNetwork crypto.Hash) ([2]uint64, error)

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
