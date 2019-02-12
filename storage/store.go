package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type Store interface {
	Close() error

	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	LoadGenesis(snapshots []*common.SnapshotWithTopologicalOrder) error
	ReadConsensusNodes() []common.Node
	CheckTransactionFinalization(hash crypto.Hash) (bool, error)
	CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error)
	ReadTransaction(hash crypto.Hash) (*common.Transaction, error)
	WriteTransaction(tx *common.Transaction) error
	StartNewRound(node crypto.Hash, number, start uint64, references [2]crypto.Hash)

	ReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	LockUTXO(hash crypto.Hash, index int, tx crypto.Hash) (*common.UTXO, error)
	CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	LockDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	CheckGhost(key crypto.Key) (bool, error)
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error)
	ReadNodesList() ([]crypto.Hash, error)
	ReadRoundMeta(nodeIdWithNetwork crypto.Hash) ([2]uint64, error)
	ReadRoundLink(from, to crypto.Hash) (uint64, error)
	WriteSnapshot(*common.SnapshotWithTopologicalOrder) error
	ReadSnapshotByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error)
	ReadDomains() []common.Domain

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
