package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type UTXO struct {
	Input
	Output
	Asset crypto.Hash
}

type UTXOWithLock struct {
	UTXO
	LockHash crypto.Hash
}

type UTXOReader interface {
	ReadUTXO(hash crypto.Hash, index int) (*UTXO, error)
	CheckDepositInput(deposit *DepositData, tx crypto.Hash) error
}

type UTXOLocker interface {
	LockUTXO(hash crypto.Hash, index int, tx crypto.Hash) (*UTXO, error)
	LockDepositInput(deposit *DepositData, tx crypto.Hash) error
}

type GhostChecker interface {
	CheckGhost(key crypto.Key) (bool, error)
}

type NodeReader interface {
	ReadConsensusNodes() []Node
	ReadTransaction(hash crypto.Hash) (*Transaction, error)
}

type DomainReader interface {
	ReadDomains() []Domain
}

type DataStore interface {
	UTXOReader
	UTXOLocker
	GhostChecker
	NodeReader
	DomainReader
}

func (tx *Transaction) UnspentOutputs() []*UTXO {
	var utxos []*UTXO
	for i, out := range tx.Outputs {
		utxo := &UTXO{
			Input: Input{
				Hash:  tx.PayloadHash(),
				Index: i,
			},
			Output: Output{
				Type:   out.Type,
				Amount: out.Amount,
				Keys:   out.Keys,
				Script: out.Script,
				Mask:   out.Mask,
			},
			Asset: tx.Asset,
		}
		utxos = append(utxos, utxo)
	}
	return utxos
}
