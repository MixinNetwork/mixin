package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type Store interface {
	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	SnapshotsLoadGenesis([]*common.SnapshotWithTopologicalOrder) error
	SnapshotsTopologySequence() uint64
	SnapshotsLockUTXO(hash crypto.Hash, index int, tx crypto.Hash, lock uint64) (*common.UTXO, error)
	SnapshotsCheckGhost(key crypto.Key) (bool, error)
	SnapshotsListTopologySince(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	SnapshotsListForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
	SnapshotsNodeList() ([]crypto.Hash, error)
	SnapshotsRoundMetaForNode(nodeIdWithNetwork crypto.Hash) ([2]uint64, error)
	SnapshotsWrite(*common.SnapshotWithTopologicalOrder) error
	SnapshotsReadByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error)

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
