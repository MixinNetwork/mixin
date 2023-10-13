package common

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

type VersionedTransaction struct {
	SignedTransaction

	pmbytes []byte
	hash    crypto.Hash
}

func (tx *SignedTransaction) AsVersioned() *VersionedTransaction {
	if tx.Version < TxVersionHashSignature {
		panic(tx.Version)
	}
	return &VersionedTransaction{
		SignedTransaction: *tx,
	}
}

func (tx *Transaction) AsVersioned() *VersionedTransaction {
	if tx.Version < TxVersionHashSignature {
		panic(tx.Version)
	}
	return &VersionedTransaction{
		SignedTransaction: SignedTransaction{Transaction: *tx},
	}
}

func UnmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	return unmarshalVersionedTransaction(val)
}

func (ver *VersionedTransaction) Marshal() []byte {
	val := ver.marshal()
	if config.Debug {
		ret, err := unmarshalVersionedTransaction(val)
		if err != nil {
			panic(err)
		}
		retv := ret.marshal()
		if !bytes.Equal(retv, val) {
			panic(fmt.Errorf("malformed %s %s", hex.EncodeToString(val), hex.EncodeToString(retv)))
		}
	}
	return val
}

func (ver *VersionedTransaction) PayloadMarshal() []byte {
	if len(ver.pmbytes) > 0 {
		return ver.pmbytes
	}
	val := ver.payloadMarshal()
	if config.Debug {
		ret, err := unmarshalVersionedTransaction(val)
		if err != nil {
			panic(err)
		}
		retv := ret.payloadMarshal()
		if !bytes.Equal(retv, val) {
			panic(fmt.Errorf("malformed %s %s", hex.EncodeToString(val), hex.EncodeToString(retv)))
		}
	}
	ver.pmbytes = val
	return ver.pmbytes
}

func (ver *VersionedTransaction) PayloadHash() crypto.Hash {
	if !ver.hash.HasValue() {
		ver.hash = crypto.Blake3Hash(ver.PayloadMarshal())
	}
	return ver.hash
}

func checkTxVersion(val []byte) uint8 {
	if len(val) < 4 {
		return 0
	}
	for _, i := range []byte{
		TxVersionHashSignature,
	} {
		v := append(magic, 0, i)
		if bytes.Equal(v, val[:4]) {
			return i
		}
	}
	return 0
}

func checkSnapVersion(val []byte) uint8 {
	if len(val) < 4 {
		return 0
	}
	for _, i := range []byte{
		SnapshotVersionCommonEncoding,
	} {
		v := append(magic, 0, i)
		if bytes.Equal(v, val[:4]) {
			return i
		}
	}
	return 0
}

func unmarshalVersionedTransaction(val []byte) (*VersionedTransaction, error) {
	if len(val) > config.TransactionMaximumSize {
		return nil, fmt.Errorf("transaction too large %d", len(val))
	}

	signed, err := NewDecoder(val).DecodeTransaction()
	if err != nil {
		return nil, err
	}
	ver := &VersionedTransaction{SignedTransaction: *signed}
	return ver, nil
}

func (ver *VersionedTransaction) marshal() []byte {
	switch ver.Version {
	case TxVersionHashSignature:
		return NewEncoder().EncodeTransaction(&ver.SignedTransaction)
	default:
		panic(ver.Version)
	}
}

func (ver *VersionedTransaction) payloadMarshal() []byte {
	switch ver.Version {
	case TxVersionHashSignature:
		signed := &SignedTransaction{Transaction: ver.Transaction}
		return NewEncoder().EncodeTransaction(signed)
	default:
		panic(ver.Version)
	}
}
