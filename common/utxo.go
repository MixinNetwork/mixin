package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

type UTXO struct {
	Input
	Output
	Asset crypto.Hash
}

type UTXOWithLock struct {
	UTXO
	LockHash crypto.Hash
}

type UTXOKeys struct {
	Mask crypto.Key
	Keys []*crypto.Key
}

func (tx *VersionedTransaction) UnspentOutputs() []*UTXOWithLock {
	var utxos []*UTXOWithLock
	hash := tx.PayloadHash()
	for i, out := range tx.Outputs {
		switch out.Type {
		case OutputTypeScript,
			OutputTypeNodePledge,
			OutputTypeNodeCancel,
			OutputTypeNodeAccept,
			OutputTypeNodeRemove,
			OutputTypeWithdrawalFuel,
			OutputTypeWithdrawalClaim,
			OutputTypeCustodianUpdateNodes:
		case OutputTypeWithdrawalSubmit,
			OutputTypeCustodianSlashNodes:
			continue
		default:
			panic(out.Type)
		}

		utxo := UTXO{
			Input: Input{
				Hash:  hash,
				Index: i,
			},
			Output: Output{
				Type:   out.Type,
				Amount: out.Amount,
				Keys:   out.Keys,
				Script: out.Script,
				Mask:   out.Mask,
			},
			Asset: tx.Asset,
		}
		utxos = append(utxos, &UTXOWithLock{UTXO: utxo})
	}
	return utxos
}

func (out *UTXOWithLock) Marshal() []byte {
	enc := NewMinimumEncoder()
	enc.Write(out.Asset[:])
	enc.EncodeInput(&out.Input)
	enc.EncodeOutput(&out.Output)
	enc.Write(out.LockHash[:])
	return enc.Bytes()
}

func UnmarshalUTXO(b []byte) (*UTXOWithLock, error) {
	if len(b) < 16 {
		return nil, fmt.Errorf("invalid UTXO size %d", len(b))
	}

	dec, err := NewMinimumDecoder(b)
	if err != nil {
		return nil, err
	}

	utxo := &UTXOWithLock{}
	err = dec.Read(utxo.Asset[:])
	if err != nil {
		return nil, err
	}

	in, err := dec.ReadInput()
	if err != nil {
		return nil, err
	}
	utxo.Input = *in

	out, err := dec.ReadOutput()
	if err != nil {
		return nil, err
	}
	utxo.Output = *out

	err = dec.Read(utxo.LockHash[:])
	return utxo, err
}
