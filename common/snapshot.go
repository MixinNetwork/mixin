package common

import (
	"github.com/MixinNetwork/mixin/crypto"
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

type Snapshot struct {
	NodeId      crypto.Hash         `json:"node"`
	Transaction crypto.Hash         `json:"transaction"`
	References  *RoundLink          `json:"references"`
	RoundNumber uint64              `json:"round"`
	Timestamp   uint64              `json:"timestamp"`
	Signatures  []*crypto.Signature `json:"signatures,omitempty"`
	Hash        crypto.Hash         `msgpack:"-"json:"hash"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64 `json:"topology"`
}

func (m *RoundLink) Equal(n *RoundLink) bool {
	return m.Self.String() == n.Self.String() && m.External.String() == n.External.String()
}

func (s *Snapshot) Payload() []byte {
	p := Snapshot{
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
		References:  s.References,
		RoundNumber: s.RoundNumber,
		Timestamp:   s.Timestamp,
	}
	return MsgpackMarshalPanic(p)
}

func (s *Snapshot) PayloadHash() crypto.Hash {
	return crypto.NewHash(s.Payload())
}

func (tx *SignedTransaction) LockInputs(locker UTXOLocker, fork bool) error {
	for _, in := range tx.Inputs {
		var err error
		if in.Mint != nil {
			err = locker.LockMintInput(in.Mint, tx.PayloadHash(), fork)
		} else if in.Deposit != nil {
			err = locker.LockDepositInput(in.Deposit, tx.PayloadHash(), fork)
		} else {
			_, err = locker.LockUTXO(in.Hash, in.Index, tx.PayloadHash(), fork)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
