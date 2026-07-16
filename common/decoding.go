package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/MixinNetwork/mixin/crypto"
)

type Decoder struct {
	buf *bytes.Reader
}

func NewDecoder(b []byte) *Decoder {
	return &Decoder{buf: bytes.NewReader(b)}
}

func NewMinimumDecoder(b []byte) (*Decoder, error) {
	if len(b) < 4 {
		return nil, fmt.Errorf("invalid encoding version %x", b)
	}
	v := append(magic, 0, MinimumEncodingVersion)
	if !bytes.Equal(v, b[:4]) {
		return nil, fmt.Errorf("invalid encoding version %x", b)
	}
	return NewDecoder(b[4:]), nil
}

func (dec *Decoder) DecodeSnapshotWithTopo() (*SnapshotWithTopologicalOrder, error) {
	b := make([]byte, 4)
	err := dec.Read(b)
	if err != nil {
		return nil, err
	}
	version := checkSnapVersion(b)
	if version < SnapshotVersionCommonEncoding {
		return nil, fmt.Errorf("invalid version %v", b)
	}

	s := &Snapshot{Version: version}

	err = dec.Read(s.NodeId[:])
	if err != nil {
		return nil, err
	}
	rn, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	s.RoundNumber = rn

	rl, err := dec.ReadRoundReferences()
	if err != nil {
		return nil, err
	}
	s.References = rl

	tl, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if tl < 1 || tl > SnapshotTransactionsMaximum {
		return nil, fmt.Errorf("invalid transactions count %d", tl)
	}
	s.Transactions = make([]crypto.Hash, tl)
	for i := range s.Transactions {
		err = dec.Read(s.Transactions[i][:])
		if err != nil {
			return nil, err
		}
	}
	for i := 1; i < len(s.Transactions); i++ {
		if bytes.Compare(s.Transactions[i-1][:], s.Transactions[i][:]) >= 0 {
			return nil, fmt.Errorf("non-canonical snapshot transaction order")
		}
	}

	if s.RoundNumber == 0 {
		if len(s.Transactions) != 1 || s.References != nil {
			return nil, fmt.Errorf("invalid transactions %d or references %v for round 0",
				len(s.Transactions), s.References)
		}
	} else if s.References == nil {
		return nil, fmt.Errorf("no references for snapshot round %d", s.RoundNumber)
	}

	ts, err := dec.ReadUint64()
	if err != nil {
		return nil, err
	}
	s.Timestamp = ts

	cs, err := dec.ReadCosiSignature()
	if err != nil {
		return nil, err
	}
	s.Signature = cs

	topo := &SnapshotWithTopologicalOrder{Snapshot: s}
	num, err := dec.ReadUint64()
	if err == io.EOF && num == 0 {
		return topo, nil
	} // genesis no signature
	topo.TopologicalOrder = num

	es, err := dec.buf.ReadByte()
	if err != io.EOF || es != 0 {
		return nil, fmt.Errorf("unexpected ending %d %v", es, err)
	}
	return topo, nil
}

func (dec *Decoder) DecodeTransaction() (*SignedTransaction, error) {
	b := make([]byte, 4)
	err := dec.Read(b)
	if err != nil {
		return nil, err
	}
	version := checkTxVersion(b)
	if version < TxVersionHashSignature {
		return nil, fmt.Errorf("invalid version %v", b)
	}

	tx := &SignedTransaction{}
	tx.Version = version

	err = dec.Read(tx.Asset[:])
	if err != nil {
		return nil, err
	}

	il, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if il > SliceCountLimit {
		return nil, fmt.Errorf("too many transaction inputs %d", il)
	}
	tx.Inputs = make([]*Input, il)
	for i := range tx.Inputs {
		in, err := dec.ReadInput()
		if err != nil {
			return nil, err
		}
		tx.Inputs[i] = in
	}

	ol, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if ol > SliceCountLimit {
		return nil, fmt.Errorf("too many transaction outputs %d", ol)
	}
	tx.Outputs = make([]*Output, ol)
	for i := range tx.Outputs {
		o, err := dec.ReadOutput()
		if err != nil {
			return nil, err
		}
		tx.Outputs[i] = o
	}

	rl, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if rl > SliceCountLimit {
		return nil, fmt.Errorf("too many transaction references %d", rl)
	}
	if rl > 0 {
		tx.References = make([]crypto.Hash, rl)
		for i := range tx.References {
			err := dec.Read(tx.References[i][:])
			if err != nil {
				return nil, err
			}
		}
	}

	el, err := dec.ReadUint32()
	if err != nil {
		return nil, err
	}
	if el > ExtraSizeStorageCapacity {
		return nil, fmt.Errorf("invalid extra size %d", el)
	}
	if el > 0 {
		b := make([]byte, el)
		err = dec.Read(b)
		if err != nil {
			return nil, err
		}
		tx.Extra = b
	}

	sl, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if sl == MaximumEncodingInt {
		prefix, err := dec.ReadInt()
		if err != nil {
			return nil, err
		}
		switch prefix {
		case AggregatedSignaturePrefix:
			js, err := dec.ReadAggregatedSignature()
			if err != nil {
				return nil, err
			}
			tx.AggregatedSignature = js
		default:
			return nil, fmt.Errorf("invalid prefix %d", prefix)
		}
	} else if sl > 0 {
		tx.SignaturesMap = make([]map[uint16]*crypto.Signature, min(sl, SliceCountLimit))
		for i := range tx.SignaturesMap {
			sm, err := dec.ReadSignatures()
			if err != nil {
				return nil, err
			}
			tx.SignaturesMap[i] = sm
		}
	}

	es, err := dec.buf.ReadByte()
	if err != io.EOF || es != 0 {
		return nil, fmt.Errorf("unexpected ending %d %v", es, err)
	}
	return tx, nil
}

func (dec *Decoder) ReadInput() (*Input, error) {
	in := &Input{}
	err := dec.Read(in.Hash[:])
	if err != nil {
		return nil, err
	}

	ii, err := dec.ReadUint16()
	if err != nil {
		return nil, err
	}
	if ii > InputIndexLimit {
		return nil, fmt.Errorf("invalid input index %d", ii)
	}
	in.Index = uint(ii)

	gb, err := dec.ReadBytes()
	if err != nil {
		return nil, err
	}
	in.Genesis = gb

	hd, err := dec.ReadMagic()
	if err != nil {
		return nil, err
	} else if hd {
		d := &DepositData{}
		err = dec.Read(d.Chain[:])
		if err != nil {
			return nil, err
		}

		ak, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		d.AssetKey = string(ak)

		th, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		d.Transaction = string(th)

		oi, err := dec.ReadUint64()
		if err != nil {
			return nil, err
		}
		d.Index = oi

		amt, err := dec.ReadInteger()
		if err != nil {
			return nil, err
		}
		d.Amount = amt
		in.Deposit = d
	}

	hm, err := dec.ReadMagic()
	if err != nil {
		return nil, err
	} else if hm {
		m := &MintData{}
		gb, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		m.Group = string(gb)

		bi, err := dec.ReadUint64()
		if err != nil {
			return nil, err
		}
		m.Batch = bi

		amt, err := dec.ReadInteger()
		if err != nil {
			return nil, err
		}
		m.Amount = amt
		in.Mint = m
	}

	return in, nil
}

func (dec *Decoder) ReadOutput() (*Output, error) {
	o := &Output{}

	var t [2]byte
	err := dec.Read(t[:])
	if err != nil {
		return nil, err
	}
	if t[0] != 0 {
		return nil, fmt.Errorf("invalid output type %v", t)
	}
	o.Type = t[1]

	amt, err := dec.ReadInteger()
	if err != nil {
		return nil, err
	}
	o.Amount = amt

	kc, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if kc > SliceCountLimit {
		return nil, fmt.Errorf("too many output keys %d", kc)
	}
	o.Keys = make([]*crypto.Key, kc)
	for i := range o.Keys {
		k := new(crypto.Key)
		err := dec.Read(k[:])
		if err != nil {
			return nil, err
		}
		o.Keys[i] = k
	}

	err = dec.Read(o.Mask[:])
	if err != nil {
		return nil, err
	}

	sb, err := dec.ReadBytes()
	if err != nil {
		return nil, err
	}
	o.Script = sb

	hw, err := dec.ReadMagic()
	if err != nil {
		return nil, err
	} else if hw {
		w := &WithdrawalData{}
		ab, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		w.Address = string(ab)

		tb, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		w.Tag = string(tb)

		o.Withdrawal = w
	}

	return o, nil
}

func (dec *Decoder) ReadSignatures() (map[uint16]*crypto.Signature, error) {
	sc, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}

	sm := make(map[uint16]*crypto.Signature, min(sc, SliceCountLimit))
	for range sc {
		si, err := dec.ReadUint16()
		if err != nil {
			return nil, err
		}
		var sig crypto.Signature
		err = dec.Read(sig[:])
		if err != nil {
			return nil, err
		}
		sm[si] = &sig
	}

	if len(sm) != sc {
		return nil, fmt.Errorf("signatures count %d %v", sc, sm)
	}
	return sm, nil
}

func (dec *Decoder) Read(b []byte) error {
	l, err := dec.buf.Read(b)
	if err != nil {
		return err
	}
	if l != len(b) {
		return fmt.Errorf("data short %d %d", l, len(b))
	}
	return nil
}

func (dec *Decoder) ReadInt() (int, error) {
	d, err := dec.ReadUint16()
	return int(d), err
}

func (dec *Decoder) ReadUint16() (uint16, error) {
	var b [2]byte
	err := dec.Read(b[:])
	if err != nil {
		return 0, err
	}
	d := binary.BigEndian.Uint16(b[:])
	if d > MaximumEncodingInt {
		return 0, fmt.Errorf("large int %d", d)
	}
	return d, nil
}

func (dec *Decoder) ReadUint32() (uint32, error) {
	var b [4]byte
	err := dec.Read(b[:])
	if err != nil {
		return 0, err
	}
	d := binary.BigEndian.Uint32(b[:])
	return d, nil
}

func (dec *Decoder) ReadUint64() (uint64, error) {
	var b [8]byte
	err := dec.Read(b[:])
	if err != nil {
		return 0, err
	}
	d := binary.BigEndian.Uint64(b[:])
	return d, nil
}

func (dec *Decoder) ReadInteger() (Integer, error) {
	il, err := dec.ReadInt()
	if err != nil {
		return Zero, err
	}
	b := make([]byte, il)
	err = dec.Read(b)
	if err != nil {
		return Zero, err
	}
	var d Integer
	d.i.SetBytes(b)
	return d, nil
}

func (dec *Decoder) ReadByte() (byte, error) {
	return dec.buf.ReadByte()
}

func (dec *Decoder) ReadBytes() ([]byte, error) {
	l, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	if l == 0 {
		return nil, nil
	}
	b := make([]byte, l)
	err = dec.Read(b)
	return b, err
}

func (dec *Decoder) ReadMagic() (bool, error) {
	var b [2]byte
	err := dec.Read(b[:])
	if err != nil {
		return false, err
	}
	if bytes.Equal(magic, b[:]) {
		return true, nil
	}
	if bytes.Equal(null, b[:]) {
		return false, nil
	}
	return false, fmt.Errorf("malformed %v", b)
}

func (dec *Decoder) ReadRoundReferences() (*RoundLink, error) {
	rc, err := dec.ReadInt()
	if err != nil || rc == 0 {
		return nil, err
	}
	if rc != 2 {
		return nil, fmt.Errorf("invalid references count %d", rc)
	}
	rl := &RoundLink{}
	err = dec.Read(rl.Self[:])
	if err != nil {
		return nil, err
	}
	err = dec.Read(rl.External[:])
	if err != nil {
		return nil, err
	}
	return rl, nil
}

func (dec *Decoder) ReadCosiSignature() (*crypto.CosiSignature, error) {
	s := &crypto.CosiSignature{}
	m, err := dec.ReadUint64()
	if err != nil || m == 0 {
		return nil, err
	}
	s.Mask = m

	err = dec.Read(s.Signature[:])
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (dec *Decoder) ReadAggregatedSignature() (*AggregatedSignature, error) {
	js := &AggregatedSignature{}
	err := dec.Read(js.Signature[:])
	if err != nil {
		return nil, err
	}

	typ, err := dec.buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch typ {
	case AggregatedSignatureSparseMask:
		l, err := dec.ReadInt()
		if err != nil {
			return nil, err
		}
		for ; l > 0; l-- {
			m, err := dec.ReadInt()
			if err != nil {
				return nil, err
			}
			js.Signers = append(js.Signers, m)
		}
	case AggregatedSignatureOrdinaryMask:
		masks, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		for i, ctr := range masks {
			for j := range byte(8) {
				k := byte(1) << j
				if ctr&k == k {
					js.Signers = append(js.Signers, i*8+int(j))
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid mask type %d", typ)
	}
	err = validateAggregatedSigners(js.Signers)
	if err != nil {
		return nil, err
	}
	return js, nil
}
