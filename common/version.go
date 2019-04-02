package common

import (
	"encoding/hex"

	"github.com/MixinNetwork/mixin/crypto"
)

type VersionedTransaction struct {
	SignedTransaction
	BadGenesis *SignedGenesisHackTransaction
}

func (tx *Transaction) AsLatestVersion() *VersionedTransaction {
	if tx.Version != TxVersion {
		panic(tx.Version)
	}
	return &VersionedTransaction{
		SignedTransaction: SignedTransaction{Transaction: *tx},
	}
}

func UnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	var tx SignedTransaction
	err := MsgpackUnmarshal(val, &tx)
	if err != nil {
		return nil, err
	}

	ver := &VersionedTransaction{
		SignedTransaction: tx,
	}

	if tx.Version == 1 && len(tx.Inputs) == 1 && hex.EncodeToString(tx.Inputs[0].Genesis) == "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997" {
		var ght SignedGenesisHackTransaction
		err := MsgpackUnmarshal(val, &ght)
		if err != nil {
			return nil, err
		}
		ver.Version = 0
		ver.BadGenesis = &ght
	}
	return ver, nil
}

func (ver *VersionedTransaction) Marshal() []byte {
	var msg []byte
	switch ver.Version {
	case 0:
		msg = MsgpackMarshalPanic(ver.BadGenesis)
	case TxVersion:
		msg = MsgpackMarshalPanic(ver.SignedTransaction)
	}
	return msg
}

func (ver *VersionedTransaction) PayloadMarshal() []byte {
	var msg []byte
	switch ver.Version {
	case 0:
		msg = MsgpackMarshalPanic(ver.BadGenesis.Transaction)
	case TxVersion:
		msg = MsgpackMarshalPanic(ver.SignedTransaction.Transaction)
	}
	return msg
}

func (ver *VersionedTransaction) PayloadHash() crypto.Hash {
	return crypto.NewHash(ver.PayloadMarshal())
}

type GenesisHackInput struct {
	Hash    crypto.Hash
	Index   int
	Genesis []byte
	Deposit *DepositData
	Rebate  []byte
	Mint    []byte
}

type SignedGenesisHackTransaction struct {
	Transaction struct {
		Version uint8
		Asset   crypto.Hash
		Inputs  []*GenesisHackInput
		Outputs []*Output
		Extra   []byte
	}
	Signatures [][]crypto.Signature
}
