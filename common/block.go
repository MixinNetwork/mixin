package common

import (
	"bytes"

	"github.com/MixinNetwork/mixin/crypto"
)

type Block struct {
	Number    uint64           // the previous block number plus one
	Timestamp uint64           // monotonical increasing timestamp
	Sequence  uint64           // all snapshots count in the blockchain
	Snapshots []crypto.Hash    // the snapshots ordered by hash
	Previous  crypto.Hash      // hash of the previous block
	NodeId    crypto.Hash      // the block producing node id
	Signature crypto.Signature // signature of all the fields above

	hash crypto.Hash
}

type BlockWithTransactions struct {
	Block
	Snapshots    map[crypto.Hash]*Snapshot
	Transactions map[crypto.Hash]*VersionedTransaction
}

func (b *Block) Verify(signer crypto.Key) bool {
	if len(b.Snapshots) > 1 {
		h := b.Snapshots[0]
		for _, s := range b.Snapshots[1:] {
			if bytes.Compare(s[:], h[:]) <= 0 {
				return false
			}
			h = s
		}
	}
	return signer.Verify(b.PayloadHash(), b.Signature)
}

func (b *Block) PayloadHash() crypto.Hash {
	if !b.hash.HasValue() {
		b.hash = crypto.Blake3Hash(b.PayloadMarshal())
	}
	return b.hash
}

func (b *Block) PayloadMarshal() []byte {
	enc := NewEncoder()
	enc.WriteUint64(b.Number)
	enc.WriteUint64(b.Timestamp)
	enc.WriteUint64(b.Sequence)
	enc.WriteInt(len(b.Snapshots))
	for _, s := range b.Snapshots {
		enc.Write(s[:])
	}
	enc.Write(b.Previous[:])
	enc.Write(b.NodeId[:])
	return enc.Bytes()
}

func (b *Block) Marshal() []byte {
	payload := b.PayloadMarshal()
	return append(payload, b.Signature[:]...)
}

func (b *BlockWithTransactions) Marshal() []byte {
	return b.MarshalWithSnapshots(b.Snapshots, b.Transactions)
}

func UnmarshalBlockWithTransactions(b []byte) (*BlockWithTransactions, error) {
	dec := NewDecoder(b)
	sl, err := dec.ReadUint16()
	if err != nil {
		return nil, err
	}
	snapshots := make(map[crypto.Hash]*Snapshot, sl)
	transactions := make(map[crypto.Hash]*VersionedTransaction)
	for range sl {
		sb, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		s, err := UnmarshalVersionedSnapshot(sb)
		if err != nil {
			return nil, err
		}
		snapshots[s.PayloadHash()] = s.Snapshot
		for _, h := range s.Transactions {
			tb, err := dec.ReadBytes()
			if err != nil {
				return nil, err
			}
			tx, err := UnmarshalVersionedTransaction(tb)
			if err != nil {
				return nil, err
			}
			transactions[h] = tx
		}
	}
	b, err = dec.ReadBytes()
	if err != nil {
		return nil, err
	}
	block, err := UnmarshalBlock(b)
	if err != nil {
		return nil, err
	}
	return &BlockWithTransactions{
		*block, snapshots, transactions,
	}, nil
}

func UnmarshalBlock(b []byte) (*Block, error) {
	dec := NewDecoder(b)
	num, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	ts, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	seq, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	sl, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	block := &Block{
		Number:    num,
		Timestamp: ts,
		Sequence:  seq,
		Snapshots: make([]crypto.Hash, sl),
	}
	for i := range sl {
		err := dec.Read(block.Snapshots[i][:])
		if err != nil {
			return nil, err
		}
	}
	err = dec.Read(block.Previous[:])
	if err != nil {
		return nil, err
	}
	err = dec.Read(block.NodeId[:])
	if err != nil {
		return nil, err
	}
	err = dec.Read(block.Signature[:])
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (b *Block) MarshalWithSnapshots(snapshots map[crypto.Hash]*Snapshot, transactions map[crypto.Hash]*VersionedTransaction) []byte {
	enc := NewEncoder()
	if len(snapshots) > 512 {
		panic(len(snapshots))
	}
	enc.WriteUint16(uint16(len(snapshots)))
	for _, id := range b.Snapshots {
		s := snapshots[id]
		b := s.VersionedMarshal()
		enc.WriteInt(len(b))
		enc.Write(b)
		for _, h := range s.Transactions {
			tx := transactions[h]
			b := tx.Marshal()
			enc.WriteInt(len(b))
			enc.Write(b)
		}
	}
	payload := b.Marshal()
	enc.WriteInt(len(payload))
	enc.Write(payload)
	return enc.Bytes()
}
