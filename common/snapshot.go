package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SnapshotVersion = 1
)

type Round struct {
	Hash       crypto.Hash
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References *RoundLink
}

type RoundLink struct {
	Self     crypto.Hash
	External crypto.Hash
}

type DeprecatedSnapshot struct {
	NodeId      crypto.Hash
	Transaction crypto.Hash
	References  *RoundLink
	RoundNumber uint64
	Timestamp   uint64
	Signatures  []*crypto.Signature
}

type Snapshot struct {
	Version     uint8
	NodeId      crypto.Hash
	Transaction crypto.Hash
	References  *RoundLink
	RoundNumber uint64
	Timestamp   uint64
	Signatures  []*crypto.Signature   `msgpack:",omitempty"`
	Signature   *crypto.CosiSignature `msgpack:",omitempty"`
	Hash        crypto.Hash           `msgpack:"-"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64
}

type SnapshotWork struct {
	Hash      crypto.Hash
	Timestamp uint64
	Signers   []crypto.Hash
}

func (m *RoundLink) Equal(n *RoundLink) bool {
	return m.Self.String() == n.Self.String() && m.External.String() == n.External.String()
}

func (m *RoundLink) Copy() *RoundLink {
	return &RoundLink{Self: m.Self, External: m.External}
}

func (s *Snapshot) VersionedPayload() []byte {
	switch s.Version {
	case 0:
		p := DeprecatedSnapshot{
			NodeId:      s.NodeId,
			Transaction: s.Transaction,
			References:  s.References,
			RoundNumber: s.RoundNumber,
			Timestamp:   s.Timestamp,
		}
		return MsgpackMarshalPanic(p)
	case SnapshotVersion:
		p := Snapshot{
			Version:     s.Version,
			NodeId:      s.NodeId,
			Transaction: s.Transaction,
			References:  s.References,
			RoundNumber: s.RoundNumber,
			Timestamp:   s.Timestamp,
		}
		return MsgpackMarshalPanic(p)
	default:
		panic(fmt.Errorf("invalid snapshot version %d", s.Version))
	}
}

func (s *Snapshot) PayloadHash() crypto.Hash {
	return crypto.NewHash(s.VersionedPayload())
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
