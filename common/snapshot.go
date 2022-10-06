package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SnapshotVersionMsgpackEncoding = 1
	SnapshotVersionCommonEncoding  = 2
)

type DeprecatedSnapshot struct {
	NodeId            crypto.Hash
	TransactionLegacy crypto.Hash `msgpack:"Transaction"`
	References        *RoundLink
	RoundNumber       uint64
	Timestamp         uint64
	Signatures        []*crypto.Signature
}

type Snapshot struct {
	Version           uint8
	NodeId            crypto.Hash
	TransactionLegacy crypto.Hash `msgpack:"Transaction"`
	References        *RoundLink
	RoundNumber       uint64
	Timestamp         uint64
	Signatures        []*crypto.Signature   `msgpack:",omitempty"`
	Signature         *crypto.CosiSignature `msgpack:",omitempty"`
	Hash              crypto.Hash           `msgpack:"-"`
	Transactions      []crypto.Hash         `msgpack:"-"`
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
		return s.TransactionLegacy
	}
	if len(s.Transactions) != 1 {
		panic(*s)
	}
	return s.Transactions[0]
}

func (s *Snapshot) AddSoleTransaction(tx crypto.Hash) {
	if s.Version < SnapshotVersionCommonEncoding {
		s.TransactionLegacy = tx
	} else if len(s.Transactions) == 0 {
		s.Transactions = []crypto.Hash{tx}
	} else {
		panic(s.Transactions[0])
	}
}

func UnmarshalVersionedSnapshot(b []byte) (*SnapshotWithTopologicalOrder, error) {
	if checkTxVersion(b) < SnapshotVersionCommonEncoding {
		var snap SnapshotWithTopologicalOrder
		err := msgpackUnmarshal(b, &snap)
		return &snap, err
	}
	return NewDecoder(b).DecodeSnapshotWithTopo()
}

func DecompressUnmarshalVersionedSnapshot(b []byte) (*SnapshotWithTopologicalOrder, error) {
	d := decompress(b)
	if d == nil {
		d = b
	}
	return UnmarshalVersionedSnapshot(d)
}

func (s *SnapshotWithTopologicalOrder) VersionedCompressMarshal() []byte {
	return compress(s.VersionedMarshal())
}

func (s *SnapshotWithTopologicalOrder) VersionedMarshal() []byte {
	switch s.Version {
	case SnapshotVersionCommonEncoding:
		return NewEncoder().EncodeSnapshotWithTopo(s)
	case 0, SnapshotVersionMsgpackEncoding:
		return msgpackMarshalPanic(s)
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
	case 0:
		p := DeprecatedSnapshot{
			NodeId:            s.NodeId,
			TransactionLegacy: s.TransactionLegacy,
			References:        s.References,
			RoundNumber:       s.RoundNumber,
			Timestamp:         s.Timestamp,
		}
		return msgpackMarshalPanic(p)
	case SnapshotVersionMsgpackEncoding:
		p := Snapshot{
			Version:           s.Version,
			NodeId:            s.NodeId,
			TransactionLegacy: s.TransactionLegacy,
			References:        s.References,
			RoundNumber:       s.RoundNumber,
			Timestamp:         s.Timestamp,
		}
		return msgpackMarshalPanic(p)
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
	p := s.versionedPayload()
	if s.Version < SnapshotVersionCommonEncoding {
		return crypto.NewHash(p)
	}
	return crypto.Blake3Hash(p)
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
