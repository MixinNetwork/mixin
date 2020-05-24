package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SnapshotVersion = 1
)

type Round struct {
	Hash       crypto.Hash `json:"hash"`
	NodeId     crypto.Hash `json:"node"`
	Number     uint64      `json:"number"`
	Timestamp  uint64      `json:"timestamp"`
	References *RoundLink  `json:"references"`
}

type RoundLink struct {
	Self     crypto.Hash `json:"self"`
	External crypto.Hash `json:"external"`
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
	Version     uint8                 `json:"version"`
	NodeId      crypto.Hash           `json:"node"`
	Transaction crypto.Hash           `json:"transaction"`
	References  *RoundLink            `json:"references"`
	RoundNumber uint64                `json:"round"`
	Timestamp   uint64                `json:"timestamp"`
	Signatures  []*crypto.Signature   `json:"signatures,omitempty" msgpack:",omitempty"`
	Signature   *crypto.CosiSignature `json:"signature,omitempty" msgpack:",omitempty"`
	Hash        crypto.Hash           `msgpack:"-" json:"hash"`
	Commitment  *crypto.Commitment    `msgpack:"-" json:"-"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64 `json:"topology"`
}

func (m *RoundLink) Equal(n *RoundLink) bool {
	return m.Self.String() == n.Self.String() && m.External.String() == n.External.String()
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
	for _, in := range tx.Inputs {
		err := locker.LockUTXO(in.Hash, in.Index, tx.PayloadHash(), fork)
		if err != nil {
			return err
		}
	}
	return nil
}
