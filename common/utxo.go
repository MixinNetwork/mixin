package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type UTXO struct {
	Input
	Output
	Asset crypto.Hash `msgpack:"C"`
}

type UTXOWithLock struct {
	UTXO
	LockHash      crypto.Hash `msgpack:"LH"`
	LockSnapshot  crypto.Hash `msgpack:"LS"`
	LockTimestamp uint64      `msgpack:"LT"`
}

type UTXOReader interface {
	SnapshotsReadUTXO(hash crypto.Hash, index int) (*UTXO, error)
	SnapshotsCheckDepositInput(deposit *DepositData, tx crypto.Hash) error
}

type UTXOLocker interface {
	SnapshotsLockUTXO(hash crypto.Hash, index int, tx, snapHash crypto.Hash, ts uint64) (*UTXO, error)
	SnapshotsLockDepositInput(deposit *DepositData, tx crypto.Hash, snapHash crypto.Hash, ts uint64) error
}

type GhostChecker interface {
	SnapshotsCheckGhost(key crypto.Key) (bool, error)
}

type NodeReader interface {
	SnapshotsReadConsensusNodes() []Node
	SnapshotsReadSnapshotByTransactionHash(hash crypto.Hash) (*SnapshotWithTopologicalOrder, error)
}

type DomainReader interface {
	SnapshotsReadDomains() []Domain
}

type DataStore interface {
	UTXOReader
	UTXOLocker
	GhostChecker
	NodeReader
	DomainReader
}

func (s *Snapshot) UnspentOutputs() []*UTXO {
	var utxos []*UTXO
	tx := s.Transaction
	if tx == nil {
		return utxos
	}

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
