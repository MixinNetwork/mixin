package common

import "github.com/MixinNetwork/mixin/crypto"

type UTXOKeysReader interface {
	ReadUTXOKeys(hash crypto.Hash, index int) (*UTXOKeys, error)
}

type UTXOLockReader interface {
	ReadUTXOLock(hash crypto.Hash, index int) (*UTXOWithLock, error)
	CheckDepositInput(deposit *DepositData, tx crypto.Hash) error
	ReadLastMintDistribution(batch uint64) (*MintDistribution, error)
}

type UTXOLocker interface {
	LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error
	LockDepositInput(deposit *DepositData, tx crypto.Hash, fork bool) error
	LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error
}

type GhostLocker interface {
	LockGhostKeys(keys []*crypto.Key, tx crypto.Hash, fork bool) error
}

type NodeReader interface {
	ReadAllNodes(offset uint64, withState bool) []*Node
	ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error)
}

type CustodianReader interface {
	ReadCustodian(ts uint64) (*CustodianUpdateRequest, error)
}

type DataStore interface {
	UTXOLockReader
	UTXOLocker
	GhostLocker
	NodeReader
	CustodianReader
}
