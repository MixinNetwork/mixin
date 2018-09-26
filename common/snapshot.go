package common

import (
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SnapshotRoundGap = uint64(3 * time.Second)
)

type ReferenceStore func(crypto.Hash) error

type Snapshot struct {
	NodeId      crypto.Hash        `msgpack:"I"json:"node"`
	Transaction *SignedTransaction `msgpack:"T"json:"transaction"`
	References  []crypto.Hash      `msgpack:"R"json:"references"` // reference to own head round hash and b peer nodes round hashes, b is 3 or 2 or 1?
	RoundNumber uint64             `msgpack:"H"json:"round"`      // if a snapshot with reference to round a confirmed, then snapshot with reference to round a-1 must never be confirmed
	Timestamp   uint64             `msgpack:"C"json:"timestamp"`
	Signatures  []crypto.Signature `msgpack:"S,omitempty"json:"signatures,omitempty"`
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

func SignSnapshot(s *Snapshot, spendKey crypto.Key) {
	msg := s.Payload()
	sig := spendKey.Sign(msg)
	for _, es := range s.Signatures {
		if es.String() == sig.String() {
			return
		}
	}
	s.Signatures = append(s.Signatures, sig)
}

func CheckSignature(s *Snapshot, pub crypto.Key) bool {
	msg := s.Payload()
	for _, sig := range s.Signatures {
		if pub.Verify(msg, sig) {
			return true
		}
	}
	return false
}

func VerifyReferences(s *Snapshot, getRef ReferenceStore) (bool, error) {
	return false, nil
}
