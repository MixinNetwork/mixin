package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type Store interface {
	Close() error

	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder) error
	ReadConsensusNodes() []common.Node
	CheckTransactionFinalization(hash crypto.Hash) (bool, error)
	CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error)
	ReadTransaction(hash crypto.Hash) (*common.Transaction, error)
	WriteTransaction(tx *common.Transaction) error
	StartNewRound(node crypto.Hash, number uint64, references [2]crypto.Hash, finalStart uint64) error
	TopologySequence() uint64

	ReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	LockUTXO(hash crypto.Hash, index int, tx crypto.Hash) (*common.UTXO, error)
	CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	LockDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	CheckGhost(key crypto.Key) (bool, error)
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadRound(hash crypto.Hash) (*common.Round, error)
	ReadLink(from, to crypto.Hash) (uint64, error)
	PruneSnapshot(snap *common.SnapshotWithTopologicalOrder) error
	WriteSnapshot(*common.SnapshotWithTopologicalOrder) error
	ReadDomains() []common.Domain

	QueueAdd(tx *common.SignedTransaction) error
	QueuePoll(uint64, func(k uint64, v []byte) error) error
}
