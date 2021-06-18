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

func (dec *Decoder) DecodeTransaction() (*SignedTransaction, error) {
	b := make([]byte, 4)
	err := dec.Read(b)
	if err != nil {
		return nil, err
	}
	if !checkTxVersion(b) {
		return nil, fmt.Errorf("invalid version %v", b)
	}

	var tx SignedTransaction
	tx.Version = TxVersion

	err = dec.Read(tx.Asset[:])
	if err != nil {
		return nil, err
	}

	il, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	for ; il > 0; il -= 1 {
		in, err := dec.ReadInput()
		if err != nil {
			return nil, err
		}
		tx.Inputs = append(tx.Inputs, in)
	}

	ol, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	for ; ol > 0; ol -= 1 {
		o, err := dec.ReadOutput()
		if err != nil {
			return nil, err
		}
		tx.Outputs = append(tx.Outputs, o)
	}

	eb, err := dec.ReadBytes()
	if err != nil {
		return nil, err
	}
	tx.Extra = eb

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
	} else {
		for ; sl > 0; sl -= 1 {
			sm, err := dec.ReadSignatures()
			if err != nil {
				return nil, err
			}
			tx.SignaturesMap = append(tx.SignaturesMap, sm)
		}
	}

	es, err := dec.buf.ReadByte()
	if err != io.EOF || es != 0 {
		return nil, fmt.Errorf("unexpected ending %d %v", es, err)
	}
	return &tx, nil
}

func (dec *Decoder) ReadInput() (*Input, error) {
	var in Input
	err := dec.Read(in.Hash[:])
	if err != nil {
		return nil, err
	}

	ii, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}
	in.Index = ii

	gb, err := dec.ReadBytes()
	if err != nil {
		return nil, err
	}
	in.Genesis = gb

	hd, err := dec.ReadMagic()
	if err != nil {
		return nil, err
	} else if hd {
		var d DepositData
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
		d.TransactionHash = string(th)

		oi, err := dec.ReadUint64()
		if err != nil {
			return nil, err
		}
		d.OutputIndex = oi

		amt, err := dec.ReadInteger()
		if err != nil {
			return nil, err
		}
		d.Amount = amt
		in.Deposit = &d
	}

	hm, err := dec.ReadMagic()
	if err != nil {
		return nil, err
	} else if hm {
		var m MintData
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
		in.Mint = &m
	}

	return &in, nil
}

func (dec *Decoder) ReadOutput() (*Output, error) {
	var o Output

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
	for ; kc > 0; kc -= 1 {
		var k crypto.Key
		err := dec.Read(k[:])
		if err != nil {
			return nil, err
		}
		o.Keys = append(o.Keys, &k)
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
		var w WithdrawalData
		err := dec.Read(w.Chain[:])
		if err != nil {
			return nil, err
		}

		ak, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		w.AssetKey = string(ak)

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

		o.Withdrawal = &w
	}

	return &o, nil
}

func (dec *Decoder) ReadSignatures() (map[uint16]*crypto.Signature, error) {
	sc, err := dec.ReadInt()
	if err != nil {
		return nil, err
	}

	sm := make(map[uint16]*crypto.Signature)
	for i := 0; i < sc; i += 1 {
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

func (dec *Decoder) ReadAggregatedSignature() (*AggregatedSignature, error) {
	var js AggregatedSignature
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
	case AggregatedSignatureOrdinayMask:
		masks, err := dec.ReadBytes()
		if err != nil {
			return nil, err
		}
		for i, ctr := range masks {
			for j := byte(0); j < 8; j++ {
				k := byte(1) << j
				if ctr&k == k {
					js.Signers = append(js.Signers, i*8+int(j))
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid mask type %d", typ)
	}
	return &js, nil
}
