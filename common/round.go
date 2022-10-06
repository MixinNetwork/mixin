package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
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

func (m *RoundLink) Equal(n *RoundLink) bool {
	return m.Self.String() == n.Self.String() && m.External.String() == n.External.String()
}

func (m *RoundLink) Copy() *RoundLink {
	return &RoundLink{Self: m.Self, External: m.External}
}

func (r *Round) CompressMarshal() []byte {
	return compress(r.Marshal())
}

func DecompressUnmarshalRound(b []byte) (*Round, error) {
	d := decompress(b)
	if d == nil {
		d = b
	}
	return UnmarshalRound(d)
}

func (r *Round) Marshal() []byte {
	enc := NewMinimumEncoder()
	enc.Write(r.Hash[:])
	enc.Write(r.NodeId[:])
	enc.WriteUint64(r.Number)
	enc.WriteUint64(r.Timestamp)
	enc.EncodeReferences(r.References)
	return enc.Bytes()
}

func UnmarshalRound(b []byte) (*Round, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("invalid round size %d", len(b))
	}

	var r Round
	dec, err := NewMinimumDecoder(b)
	if err != nil {
		err := msgpackUnmarshal(b, &r)
		return &r, err
	}

	err = dec.Read(r.Hash[:])
	if err != nil {
		return nil, err
	}

	err = dec.Read(r.NodeId[:])
	if err != nil {
		return nil, err
	}
	num, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	r.Number = num

	ts, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	r.Timestamp = ts

	link, err := dec.ReadReferences()
	if err != nil {
		return nil, err
	}
	r.References = link
	return &r, err
}
