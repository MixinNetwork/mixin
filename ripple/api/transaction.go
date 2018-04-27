package api

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/rubblelabs/ripple/crypto"
	"github.com/rubblelabs/ripple/data"
	"mixin.one/number"
)

const (
	assetKeyXRP       = "23dfb5a5-5d7b-48b6-905f-3970e3176e27"
	assetPrecisionXRP = 6
)

type Asset struct {
	Key       string
	Precision int32
}

type rippleKey struct {
	*btcec.PrivateKey
}

func (k *rippleKey) Id(sequence *uint32) []byte {
	return crypto.Sha256RipeMD160(k.PubKey().SerializeCompressed())
}

func (k *rippleKey) Public(sequence *uint32) []byte {
	return k.PubKey().SerializeCompressed()
}

func (k *rippleKey) Private(sequence *uint32) []byte {
	return k.D.Bytes()
}

// If transaction queues, fee averaging helps, so we don't need LastLedgerSequence, but the queue can have at most 10 transactions
func LocalSignRawTransaction(asset *Asset, receiver string, amount, fee number.Decimal, privateKey string, sequence uint32) (string, string, error) {
	if asset.Key != assetKeyXRP {
		return "", "", fmt.Errorf("Ripple asset not supported %s", asset.Key)
	}
	if asset.Precision != assetPrecisionXRP {
		return "", "", fmt.Errorf("Ripple invalid XRP precision %d", asset.Precision)
	}

	receiver, err := LocalNormalizePublicKey(receiver)
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid receiver %s", err.Error())
	}

	destinationHash, err := crypto.NewRippleHash(receiver)
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid destination %s", err.Error())
	}
	var destination data.Account
	if len(destinationHash.Payload()) != len(destination) {
		return "", "", fmt.Errorf("Ripple invalid destination %s", receiver)
	}
	copy(destination[:], destinationHash.Payload())

	rippleAmount, err := data.NewAmount(int64(amount.Mul(number.New(1, -asset.Precision)).Floor().Float64()))
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid amount %s", err.Error())
	}

	key, err := hex.DecodeString(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid private key %s", err.Error())
	}
	cryptoKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), key)

	var sender data.Account
	senderHash, err := crypto.NewAccountId(crypto.Sha256RipeMD160(publicKey.SerializeCompressed()))
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid sender %s", err.Error())
	}
	if len(senderHash.Payload()) != len(sender) {
		return "", "", fmt.Errorf("Ripple invalid sender %s", senderHash.String())
	}
	copy(sender[:], senderHash.Payload())

	feeValue, err := data.NewNativeValue(int64(fee.Mul(number.New(1, -assetPrecisionXRP)).Floor().Float64()))
	if err != nil {
		return "", "", fmt.Errorf("Ripple invalid fee %s", err.Error())
	}

	txn := &data.Payment{
		TxBase: data.TxBase{
			TransactionType: data.PAYMENT,
			Account:         sender,
			Fee:             *feeValue,
			Sequence:        sequence,
		},
		Destination: destination,
		Amount:      *rippleAmount,
	}

	err = data.Sign(txn, &rippleKey{cryptoKey}, nil)
	if err != nil {
		return "", "", fmt.Errorf("Ripple signer failed %s", err.Error())
	}
	rawHash, raw, err := data.Raw(txn)
	if err != nil {
		return "", "", fmt.Errorf("Ripple signer failed %s", err.Error())
	}
	return rawHash.String(), hex.EncodeToString(raw), nil
}
