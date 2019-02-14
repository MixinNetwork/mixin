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
	Transaction *SignedTransaction `json:"transaction"`
	References  [2]crypto.Hash     `json:"references"`
	RoundNumber uint64             `json:"round"`
	Timestamp   uint64             `json:"timestamp"`
	Signatures  []crypto.Signature `json:"signatures,omitempty"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64      `msgpack:"-"json:"topology"`
	Hash             crypto.Hash `msgpack:"-"json:"hash"`
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

func (s *Snapshot) LockInputs(locker UTXOLocker) error {
	txHash := s.Transaction.PayloadHash()
	for _, in := range s.Transaction.Inputs {
		var err error
		if in.Deposit != nil {
			err = locker.LockDepositInput(in.Deposit, txHash)
		} else {
			_, err = locker.LockUTXO(in.Hash, in.Index, txHash)
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
