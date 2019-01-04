package common

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type Snapshot struct {
	NodeId      crypto.Hash        `msgpack:"I"json:"node"`
	Transaction *SignedTransaction `msgpack:"T"json:"transaction"`
	References  [2]crypto.Hash     `msgpack:"R"json:"references"`
	RoundNumber uint64             `msgpack:"H"json:"round"`
	Timestamp   uint64             `msgpack:"C"json:"timestamp"`
	Signatures  []crypto.Signature `msgpack:"S,omitempty"json:"signatures,omitempty"`
}

type SnapshotWithTopologicalOrder struct {
	Snapshot
	TopologicalOrder uint64                 `msgpack:"-"json:"topology"`
	Hash             crypto.Hash            `msgpack:"-"json:"hash"`
	RoundLinks       map[crypto.Hash]uint64 `msgpack:"-"json:"-"`
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

func (s *Snapshot) Validate(readUTXO UTXOReader, checkGhost GhostChecker, lockUTXOForTransaction UTXOLocker) error {
	err := s.Transaction.Validate(readUTXO, checkGhost)
	if err != nil {
		return err
	}

	tx := s.Transaction
	for _, in := range tx.Inputs {
		_, err := lockUTXOForTransaction(in.Hash, in.Index, tx.PayloadHash(), s.PayloadHash(), s.Timestamp)
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
