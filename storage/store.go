package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type Store interface {
	Close() error

	StateGet(key string, val interface{}) (bool, error)
	StateSet(key string, val interface{}) error

	CheckGenesisLoad() (bool, error)
	LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.SignedTransaction) error
	ReadConsensusNodes() []*common.Node
	CheckTransactionFinalization(hash crypto.Hash) (bool, error)
	CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error)
	ReadTransaction(hash crypto.Hash) (*common.SignedTransaction, error)
	WriteTransaction(tx *common.SignedTransaction) error
	StartNewRound(node crypto.Hash, number uint64, references *common.RoundLink, finalStart uint64) error
	TopologySequence() uint64

	ReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error)
	LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) (*common.UTXO, error)
	CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	LockDepositInput(deposit *common.DepositData, tx crypto.Hash, fork bool) error
	CheckGhost(key crypto.Key) (bool, error)
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadRound(hash crypto.Hash) (*common.Round, error)
	ReadLink(from, to crypto.Hash) (uint64, error)
	WriteSnapshot(*common.SnapshotWithTopologicalOrder) error
	ReadDomains() []common.Domain

	QueueInfo() (uint64, uint64, error)
	QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot, finalized bool) error
	QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error)
	CachePutTransaction(tx *common.SignedTransaction) error
	CacheGetTransaction(hash crypto.Hash) (*common.SignedTransaction, error)
	CacheListTransactions(hook func(tx *common.SignedTransaction) error) error
}
