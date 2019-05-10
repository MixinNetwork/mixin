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
	LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.VersionedTransaction) error
	ReadConsensusNodes() []*common.Node
	CheckTransactionFinalization(hash crypto.Hash) (bool, error)
	CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error)
	ReadTransaction(hash crypto.Hash) (*common.VersionedTransaction, error)
	WriteTransaction(tx *common.VersionedTransaction) error
	StartNewRound(node crypto.Hash, number uint64, references *common.RoundLink, finalStart uint64) error
	UpdateEmptyHeadRound(node crypto.Hash, number uint64, references *common.RoundLink) error
	TopologySequence() uint64

	ReadUTXO(hash crypto.Hash, index int) (*common.UTXOWithLock, error)
	LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) error
	CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error
	LockDepositInput(deposit *common.DepositData, tx crypto.Hash, fork bool) error
	CheckGhost(key crypto.Key) (bool, error)
	ReadSnapshot(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotWithTransactionsSinceTopology(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, []*common.VersionedTransaction, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadRound(hash crypto.Hash) (*common.Round, error)
	ReadLink(from, to crypto.Hash) (uint64, error)
	WriteSnapshot(*common.SnapshotWithTopologicalOrder) error
	ReadDomains() []common.Domain

	QueueInfo() (uint64, uint64, uint64, error)
	QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot, finalized bool) error
	QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error)
	CachePutTransaction(tx *common.VersionedTransaction) error
	CacheGetTransaction(hash crypto.Hash) (*common.VersionedTransaction, error)
	CacheListTransactions(hook func(tx *common.VersionedTransaction) error) error

	ReadLastMintDistribution(group string) (*common.MintDistribution, error)
	LockMintInput(mint *common.MintData, tx crypto.Hash, fork bool) error
	ReadMintDistributions(group string, offset, count uint64) ([]*common.MintDistribution, []*common.VersionedTransaction, error)

	RemoveGraphEntries(prefix string) error
	ValidateGraphEntries() (int, int, error)

	WriteAsset(a *common.Asset) error
	ReadAsset(id crypto.Hash) (*common.Asset, error)
}
