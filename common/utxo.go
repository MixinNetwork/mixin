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

type UTXOKeysReader interface {
	ReadUTXOKeys(hash crypto.Hash, index int) (*UTXOKeys, error)
}

type UTXOLockReader interface {
	ReadUTXOLock(hash crypto.Hash, index int) (*UTXOWithLock, error)
	CheckDepositInput(deposit *DepositData, tx crypto.Hash) error
	ReadLastMintDistribution(group string) (*MintDistribution, error)
}

type UTXOLocker interface {
	LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error
	LockDepositInput(deposit *DepositData, tx crypto.Hash, fork bool) error
	LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error
}

type GhostChecker interface {
	CheckGhost(key crypto.Key) (*crypto.Hash, error)
}

type NodeReader interface {
	ReadAllNodes(offset uint64, withState bool) []*Node
	ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error)
}

type DomainReader interface {
	ReadDomains() []*Domain
}

type DataStore interface {
	UTXOLockReader
	UTXOLocker
	GhostChecker
	NodeReader
	DomainReader
}

func (tx *VersionedTransaction) UnspentOutputs() []*UTXOWithLock {
	var utxos []*UTXOWithLock
	for i, out := range tx.Outputs {
		switch out.Type {
		case OutputTypeScript,
			OutputTypeNodePledge,
			OutputTypeNodeCancel,
			OutputTypeNodeAccept,
			OutputTypeNodeRemove,
			OutputTypeDomainAccept,
			OutputTypeWithdrawalFuel,
			OutputTypeWithdrawalClaim:
		case OutputTypeWithdrawalSubmit:
			continue
		default:
			panic(out.Type)
		}

		utxo := UTXO{
			Input: Input{
				Hash:  tx.PayloadHash(),
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

func (out *UTXOWithLock) CompressMarshal() []byte {
	return compress(out.Marshal())
}

func DecompressUnmarshalUTXO(b []byte) (*UTXOWithLock, error) {
	d := decompress(b)
	if d == nil {
		d = b
	}
	return UnmarshalUTXO(d)
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

	var utxo UTXOWithLock
	dec, err := NewMinimumDecoder(b)
	if err != nil {
		err := msgpackUnmarshal(b, &utxo)
		return &utxo, err
	}

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
	return &utxo, err
}
