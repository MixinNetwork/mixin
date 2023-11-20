package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/config"
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

func (r *Round) Marshal() []byte {
	enc := NewMinimumEncoder()
	enc.Write(r.Hash[:])
	enc.Write(r.NodeId[:])
	enc.WriteUint64(r.Number)
	enc.WriteUint64(r.Timestamp)
	enc.EncodeRoundReferences(r.References)
	return enc.Bytes()
}

func ComputeRoundHash(nodeId crypto.Hash, number uint64, snapshots []*Snapshot) (uint64, uint64, crypto.Hash) {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Timestamp < snapshots[j].Timestamp {
			return true
		}
		if snapshots[i].Timestamp > snapshots[j].Timestamp {
			return false
		}
		a, b := snapshots[i].Hash, snapshots[j].Hash
		return bytes.Compare(a[:], b[:]) < 0
	})
	start := snapshots[0].Timestamp
	end := snapshots[len(snapshots)-1].Timestamp
	if end >= start+config.SnapshotRoundGap {
		err := fmt.Errorf("ComputeRoundHash(%s, %d) %d %d %d", nodeId, number, start, end, start+config.SnapshotRoundGap)
		panic(err)
	}

	version := snapshots[0].Version
	for _, s := range snapshots {
		if s.Version > version {
			version = s.Version
		}
	}

	buf := binary.BigEndian.AppendUint64(nodeId[:], number)
	hash := crypto.Blake3Hash(buf)
	for _, s := range snapshots {
		if s.Version > version {
			panic(nodeId)
		}
		if s.Timestamp > end {
			panic(nodeId)
		}
		hash = crypto.Blake3Hash(append(hash[:], s.Hash[:]...))
	}
	return start, end, hash
}

func UnmarshalRound(b []byte) (*Round, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("invalid round size %d", len(b))
	}

	r := &Round{}
	dec, err := NewMinimumDecoder(b)
	if err != nil {
		return nil, err
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

	link, err := dec.ReadRoundReferences()
	if err != nil {
		return nil, err
	}
	r.References = link
	return r, err
}
