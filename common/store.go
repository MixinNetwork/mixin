package common

import "github.com/MixinNetwork/mixin/crypto"

type UTXOKeysReader interface {
	ReadUTXOKeys(hash crypto.Hash, index int) (*UTXOKeys, error)
}

type UTXOLockReader interface {
	ReadUTXOLock(hash crypto.Hash, index int) (*UTXOWithLock, error)
	CheckDepositInput(deposit *DepositData, tx crypto.Hash) error
	ReadLastMintDistribution(ts uint64) (*MintDistribution, error)
}

type UTXOLocker interface {
	LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error
	LockDepositInput(deposit *DepositData, tx crypto.Hash, fork bool) error
	LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error
}

type GhostChecker interface {
	CheckGhost(key crypto.Key) (*crypto.Hash, error)
}

type NodeReader interface {
	ReadAllNodes(offset uint64, withState bool) []*Node
	ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error)
}

type DomainReader interface {
	ReadDomains() []*Domain
}

type CustodianReader interface {
	ReadCustodianAccount(ts uint64) (*Address, uint64, error)
	ReadCustodianNodes(ts uint64) ([]*CustodianNode, error)
}

type DataStore interface {
	UTXOLockReader
	UTXOLocker
	GhostChecker
	NodeReader
	DomainReader
	CustodianReader
}
