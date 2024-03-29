package common

import "github.com/MixinNetwork/mixin/crypto"

type TransactionReader interface {
	ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error)
}

type UTXOKeysReader interface {
	ReadUTXOKeys(hash crypto.Hash, index uint) (*UTXOKeys, error)
}

type UTXOLockReader interface {
	ReadUTXOLock(hash crypto.Hash, index uint) (*UTXOWithLock, error)
	ReadDepositLock(deposit *DepositData) (crypto.Hash, error)
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
}

type CustodianReader interface {
	ReadCustodian(ts uint64) (*CustodianUpdateRequest, error)
}

type AssetReader interface {
	ReadAssetWithBalance(id crypto.Hash) (*Asset, Integer, error)
}

type DataStore interface {
	TransactionReader
	UTXOLockReader
	UTXOLocker
	GhostLocker
	NodeReader
	CustodianReader
	AssetReader
}
