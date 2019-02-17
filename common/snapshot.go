package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type Round struct {
	Hash       crypto.Hash    `json:"hash"`
	NodeId     crypto.Hash    `json:"node"`
	Number     uint64         `json:"number"`
	Timestamp  uint64         `json:"timestamp"`
	References [2]crypto.Hash `json:"references"`
}

type Snapshot struct {
	NodeId      crypto.Hash        `json:"node"`
	Transaction crypto.Hash        `json:"transaction"`
	References  [2]crypto.Hash     `json:"references"`
	RoundNumber uint64             `json:"round"`
	Timestamp   uint64             `json:"timestamp"`
	Signatures  []crypto.Signature `json:"signatures,omitempty"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64 `json:"topology"`
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
		if in.Deposit != nil {
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

func (s *Snapshot) Sign(spendKey crypto.Key) {
	msg := s.Payload()
	sig := spendKey.Sign(msg)
	for _, es := range s.Signatures {
		if es == sig {
			return
		}
	}
	s.Signatures = append(s.Signatures, sig)
}

func (s *Snapshot) CheckSignature(pub crypto.Key) bool {
	msg := s.Payload()
	for _, sig := range s.Signatures {
		if pub.Verify(msg, sig) {
			return true
		}
	}
	return false
}
