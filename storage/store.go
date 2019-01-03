package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type Store interface {
	Close() error

	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	SnapshotsLoadGenesis([]*common.SnapshotWithTopologicalOrder) error
	SnapshotsTopologySequence() uint64
	SnapshotsReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	SnapshotsLockUTXO(hash crypto.Hash, index int, tx crypto.Hash) (*common.UTXO, error)
	SnapshotsCheckGhost(key crypto.Key) (bool, error)
	SnapshotsReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	SnapshotsReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
	SnapshotsReadNodesList() ([]crypto.Hash, error)
	SnapshotsReadRoundMeta(nodeIdWithNetwork crypto.Hash) ([2]uint64, error)
	SnapshotsReadRoundLink(from, to crypto.Hash) (uint64, error)
	SnapshotsWriteSnapshot(*common.SnapshotWithTopologicalOrder) error
	SnapshotsReadSnapshotByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error)

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
