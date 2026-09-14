package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MinimumEncodingVersion = 0x1
	MaximumEncodingInt     = 0xFFFF

	AggregatedSignaturePrefix       = 0xFF01
	AggregatedSignatureSparseMask   = byte(0x01)
	AggregatedSignatureOrdinaryMask = byte(0x00)
)

var (
	magic = []byte{0x77, 0x77}
	null  = []byte{0x00, 0x00}
)

type Encoder struct {
	buf []byte
}

func NewEncoder() *Encoder {
	return new(Encoder)
}

func newEncoder(capacity int) *Encoder {
	return &Encoder{buf: make([]byte, 0, capacity)}
}

func NewMinimumEncoder() *Encoder {
	enc := NewEncoder()
	enc.Write(magic)
	enc.Write([]byte{0x00, MinimumEncodingVersion})
	return enc
}

func (enc *Encoder) Bytes() []byte {
	return enc.buf
}

func (enc *Encoder) EncodeSnapshotWithTopo(s *SnapshotWithTopologicalOrder) []byte {
	enc.encodeSnapshotPayload(s.Snapshot, true)
	enc.WriteUint64(s.TopologicalOrder)
	return enc.Bytes()
}

func (enc *Encoder) EncodeSnapshotPayload(s *Snapshot) []byte {
	enc.encodeSnapshotPayload(s, false)
	return enc.Bytes()
}

func (enc *Encoder) encodeSnapshotPayload(s *Snapshot, withSig bool) {
	if s.Version < SnapshotVersionCommonEncoding {
		panic(s)
	}
	if s.RoundNumber == 0 && len(s.Transactions) != 1 {
		panic(len(s.Transactions))
	}
	if l := len(s.Transactions); l < 1 || l > SnapshotTransactionsMaximum {
		panic(fmt.Errorf("invalid transactions count %d", l))
	}
	if !withSig && s.Signature != nil {
		panic(s.Signature)
	}

	enc.Write(magic)
	enc.Write([]byte{0x00, s.Version})
	enc.Write(s.NodeId[:])
	enc.WriteUint64(s.RoundNumber)

	enc.EncodeRoundReferences(s.References)

	enc.WriteInt(len(s.Transactions))
	slices.SortFunc(s.Transactions, func(a, b crypto.Hash) int {
		return bytes.Compare(a[:], b[:])
	})
	for i := 1; i < len(s.Transactions); i++ {
		if s.Transactions[i-1] == s.Transactions[i] {
			panic(fmt.Errorf("duplicate snapshot transaction %s", s.Transactions[i]))
		}
	}
	for _, t := range s.Transactions {
		enc.Write(t[:])
	}

	enc.WriteUint64(s.Timestamp)
	enc.EncodeCosiSignature(s.Signature)
}

func (enc *Encoder) EncodeTransaction(signed *SignedTransaction) []byte {
	if signed.Version < TxVersionHashSignature {
		panic(signed)
	}

	enc.Write(magic)
	enc.Write([]byte{0x00, signed.Version})
	enc.Write(signed.Asset[:])

	il := len(signed.Inputs)
	if il > SliceCountLimit {
		panic(il)
	}
	enc.WriteInt(il)
	for _, in := range signed.Inputs {
		enc.EncodeInput(in)
	}

	ol := len(signed.Outputs)
	if ol > SliceCountLimit {
		panic(ol)
	}
	enc.WriteInt(ol)
	for _, out := range signed.Outputs {
		enc.EncodeOutput(out)
	}

	rl := len(signed.References)
	enc.WriteInt(rl)
	for _, r := range signed.References {
		enc.Write(r[:])
	}

	el := len(signed.Extra)
	if el > ExtraSizeStorageCapacity {
		panic(el)
	}
	enc.WriteUint32(uint32(el))
	enc.Write(signed.Extra)

	if signed.AggregatedSignature != nil {
		enc.EncodeAggregatedSignature(signed.AggregatedSignature)
	} else {
		sl := len(signed.SignaturesMap)
		if sl == MaximumEncodingInt {
			panic(sl)
		}
		enc.WriteInt(sl)
		for _, sm := range signed.SignaturesMap {
			enc.EncodeSignatures(sm)
		}
	}

	return enc.Bytes()
}

func (enc *Encoder) EncodeInput(in *Input) {
	if in.Index > InputIndexLimit {
		panic(in.Index)
	}
	enc.Write(in.Hash[:])
	enc.WriteUint16(uint16(in.Index))

	enc.WriteInt(len(in.Genesis))
	enc.Write(in.Genesis)

	if d := in.Deposit; d == nil {
		enc.Write(null)
	} else {
		enc.Write(magic)
		enc.Write(d.Chain[:])

		enc.WriteInt(len(d.AssetKey))
		enc.Write([]byte(d.AssetKey))

		enc.WriteInt(len(d.Transaction))
		enc.Write([]byte(d.Transaction))

		enc.WriteUint64(d.Index)
		enc.WriteInteger(d.Amount)
	}

	if m := in.Mint; m == nil {
		enc.Write(null)
	} else {
		enc.Write(magic)

		enc.WriteInt(len(m.Group))
		enc.Write([]byte(m.Group))

		enc.WriteUint64(m.Batch)
		enc.WriteInteger(m.Amount)
	}
}

func (enc *Encoder) EncodeOutput(o *Output) {
	enc.Write([]byte{0x00, o.Type})
	enc.WriteInteger(o.Amount)
	enc.WriteInt(len(o.Keys))
	for _, k := range o.Keys {
		enc.Write(k[:])
	}

	enc.Write(o.Mask[:])
	enc.WriteInt(len(o.Script))
	enc.Write(o.Script)

	if w := o.Withdrawal; w == nil {
		enc.Write(null)
	} else {
		enc.Write(magic)

		enc.WriteInt(len(w.Address))
		enc.Write([]byte(w.Address))

		enc.WriteInt(len(w.Tag))
		enc.Write([]byte(w.Tag))
	}
}

func (enc *Encoder) EncodeSignatures(sm map[uint16]*crypto.Signature) {
	ss, off := make([]struct {
		Index uint16
		Sig   *crypto.Signature
	}, len(sm)), 0
	for j, sig := range sm {
		ss[off].Index = j
		ss[off].Sig = sig
		off += 1
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Index < ss[j].Index })

	enc.WriteInt(len(ss))
	for _, sp := range ss {
		enc.WriteUint16(sp.Index)
		enc.Write(sp.Sig[:])
	}
}

func (enc *Encoder) Write(b []byte) {
	enc.buf = append(enc.buf, b...)
}

func (enc *Encoder) WriteByte(b byte) error {
	enc.buf = append(enc.buf, b)
	return nil
}

func (enc *Encoder) WriteInt(d int) {
	if d > MaximumEncodingInt {
		panic(d)
	}
	enc.WriteUint16(uint16(d))
}

func (enc *Encoder) WriteUint16(d uint16) {
	if d > MaximumEncodingInt {
		panic(d)
	}
	enc.buf = binary.BigEndian.AppendUint16(enc.buf, d)
}

func (enc *Encoder) WriteUint32(d uint32) {
	enc.buf = binary.BigEndian.AppendUint32(enc.buf, d)
}

func (enc *Encoder) WriteUint64(d uint64) {
	enc.buf = binary.BigEndian.AppendUint64(enc.buf, d)
}

func (enc *Encoder) WriteInteger(d Integer) {
	size := (d.i.BitLen() + 7) / 8
	enc.WriteInt(size)
	start := len(enc.buf)
	enc.buf = slices.Grow(enc.buf, size)[:start+size]
	d.i.FillBytes(enc.buf[start:])
}

func (enc *Encoder) EncodeRoundReferences(r *RoundLink) {
	if r == nil { // genesis
		enc.WriteInt(0)
	} else {
		enc.WriteInt(2)
		enc.Write(r.Self[:])
		enc.Write(r.External[:])
	}
}

func (enc *Encoder) EncodeCosiSignature(s *crypto.CosiSignature) {
	if s == nil {
		enc.WriteUint64(0)
		return
	}

	if s.Mask == 0 {
		panic(s.Signature)
	}
	enc.WriteUint64(s.Mask)
	enc.Write(s.Signature[:])
}

func (enc *Encoder) EncodeAggregatedSignature(js *AggregatedSignature) {
	enc.WriteInt(MaximumEncodingInt)
	enc.WriteInt(AggregatedSignaturePrefix)
	enc.Write(js.Signature[:])
	if len(js.Signers) == 0 {
		_ = enc.WriteByte(AggregatedSignatureOrdinaryMask)
		enc.WriteInt(0)
		return
	}
	err := validateAggregatedSigners(js.Signers)
	if err != nil {
		panic(err)
	}

	max := js.Signers[len(js.Signers)-1]
	if max/8+1 > len(js.Signers)*2 {
		_ = enc.WriteByte(AggregatedSignatureSparseMask)
		enc.WriteInt(len(js.Signers))
		for _, m := range js.Signers {
			enc.WriteInt(m)
		}
		return
	}

	masks := make([]byte, max/8+1)
	for _, m := range js.Signers {
		masks[m/8] = masks[m/8] ^ (1 << (m % 8))
	}
	_ = enc.WriteByte(AggregatedSignatureOrdinaryMask)
	enc.WriteInt(len(masks))
	enc.Write(masks)
}

type AggregatedSignature struct {
	Signers   []int
	Signature crypto.Signature
}

func validateAggregatedSigners(signers []int) error {
	if len(signers) > MaximumEncodingInt {
		return fmt.Errorf("too many aggregated signers %d", len(signers))
	}
	prev := -1
	for _, signer := range signers {
		if signer <= prev || signer > MaximumEncodingInt {
			return fmt.Errorf("invalid aggregated signer order %d <= %d", signer, prev)
		}
		prev = signer
	}
	return nil
}
