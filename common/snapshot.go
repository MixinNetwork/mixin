package common

import (
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SnapshotVersionCommonEncoding = 2
)

type Snapshot struct {
	Version uint8
	NodeId  crypto.Hash
	// TODO
	// after the blockchain is ready, we will remove round, the snapshot just reference
	// previous snapshot in this chain, and a block hash, no round should be
	// much faster
	References   *RoundLink // one previous round, the external will be block hash, then no block vote snapshot required
	RoundNumber  uint64
	Timestamp    uint64
	Signature    *crypto.CosiSignature
	Hash         crypto.Hash
	Transactions []crypto.Hash
}

type SnapshotWithTopologicalOrder struct {
	*Snapshot
	TopologicalOrder uint64
}

type SnapshotWork struct {
	Hash      crypto.Hash
	Timestamp uint64
	Signers   []crypto.Hash
}

func (s *Snapshot) SoleTransaction() crypto.Hash {
	if s.Version < SnapshotVersionCommonEncoding {
		panic(s.Version)
	}
	if len(s.Transactions) != 1 {
		panic(len(s.Transactions))
	}
	return s.Transactions[0]
}

func (s *Snapshot) AddSoleTransaction(tx crypto.Hash) {
	if s.Version < SnapshotVersionCommonEncoding {
		panic(s.Version)
	} else if len(s.Transactions) == 0 {
		s.Transactions = []crypto.Hash{tx}
	} else {
		panic(s.Transactions[0])
	}
}

func UnmarshalVersionedSnapshot(b []byte) (*SnapshotWithTopologicalOrder, error) {
	if checkSnapVersion(b) < SnapshotVersionCommonEncoding {
		panic(hex.EncodeToString(b))
	}
	return NewDecoder(b).DecodeSnapshotWithTopo()
}

func (s *SnapshotWithTopologicalOrder) VersionedMarshal() []byte {
	switch s.Version {
	case SnapshotVersionCommonEncoding:
		return NewEncoder().EncodeSnapshotWithTopo(s)
	default:
		panic(s.Version)
	}
}

func (s *Snapshot) VersionedMarshal() []byte {
	topo := &SnapshotWithTopologicalOrder{Snapshot: s}
	return topo.VersionedMarshal()
}

func (s *Snapshot) versionedPayload() []byte {
	switch s.Version {
	case SnapshotVersionCommonEncoding:
		p := &Snapshot{
			Version:      s.Version,
			NodeId:       s.NodeId,
			RoundNumber:  s.RoundNumber,
			References:   s.References,
			Transactions: s.Transactions,
			Timestamp:    s.Timestamp,
		}
		return NewEncoder().EncodeSnapshotPayload(p)
	default:
		panic(fmt.Errorf("invalid snapshot version %d", s.Version))
	}
}

func (s *Snapshot) PayloadHash() crypto.Hash {
	if !s.Hash.HasValue() {
		p := s.versionedPayload()
		if s.Version < SnapshotVersionCommonEncoding {
			panic(s.Version)
		}
		s.Hash = crypto.Blake3Hash(p)
	}
	return s.Hash
}

func (tx *VersionedTransaction) LockInputs(locker UTXOLocker, fork bool) error {
	switch tx.TransactionType() {
	case TransactionTypeMint:
		return locker.LockMintInput(tx.Inputs[0].Mint, tx.PayloadHash(), fork)
	case TransactionTypeDeposit:
		return locker.LockDepositInput(tx.Inputs[0].Deposit, tx.PayloadHash(), fork)
	}
	return locker.LockUTXOs(tx.Inputs, tx.PayloadHash(), fork)
}
