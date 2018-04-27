package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/cpacia/bchutil"
)

type UTXO struct {
	TransactionHash string
	Index           uint32
	Amount          int64
	PrivateKey      string
}

func LocalNormalizePublicKey(address string) (string, error) {
	address = strings.TrimSpace(address)
	btcAddress, err := btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err != nil {
		return "", err
	}
	if btcAddress.String() != address {
		return "", fmt.Errorf("Bitcoin Cash NormalizeAddress mismatch %s", address)
	}
	return btcAddress.String(), nil
}

func LocalEstimateTransactionFee(inputs []*UTXO, feePerKb int64) int64 {
	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	return feePerKb * estimatedRawSizeInKb
}

func LocalSignRawTransaction(inputs []*UTXO, output string, amount int64, feePerKb int64, changeAddress string) (string, string, int64, error) {
	tx, inputAmount := wire.NewMsgTx(wire.TxVersion), int64(0)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.TransactionHash)
		if err != nil {
			return "", "", 0, err
		}
		txIn := &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  *hash,
				Index: input.Index,
			},
			Sequence: 0xffffffff,
		}
		tx.AddTxIn(txIn)
		inputAmount = inputAmount + input.Amount
	}

	addressPubKeyHash, err := btcutil.DecodeAddress(output, &chaincfg.MainNetParams)
	if err != nil {
		return "", "", 0, err
	}
	pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
	if err != nil {
		return "", "", 0, err
	}
	tx.AddTxOut(wire.NewTxOut(amount, pkScript))

	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	feeToConsumed := feePerKb * estimatedRawSizeInKb
	changeAmount := inputAmount - feeToConsumed - amount
	if changeAmount < 0 {
		return "", "", 0, fmt.Errorf("insuficcient trasaction fee %d %d %d", inputAmount, feePerKb, estimatedRawSizeInKb)
	}
	if changeAmount > feePerKb {
		addressPubKeyHash, err := btcutil.DecodeAddress(changeAddress, &chaincfg.MainNetParams)
		if err != nil {
			return "", "", 0, err
		}
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return "", "", 0, err
		}
		tx.AddTxOut(wire.NewTxOut(changeAmount, pkScript))
	} else {
		feeToConsumed = inputAmount - amount
	}

	for idx, input := range inputs {
		privateKeyBytes, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return "", "", 0, err
		}
		privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
		if err != nil {
			return "", "", 0, err
		}
		addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
		if err != nil {
			return "", "", 0, err
		}
		addressPubKeyHash := addressPubKey.AddressPubKeyHash()
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return "", "", 0, err
		}
		sigScript, err := bchutil.SignatureScript(tx, idx, pkScript, txscript.SigHashAll, privateKey, true, input.Amount)
		if err != nil {
			return "", "", 0, err
		}
		tx.TxIn[idx].SignatureScript = sigScript
	}

	var rawBuffer bytes.Buffer
	err = tx.BtcEncode(&rawBuffer, wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return "", "", 0, err
	}
	rawBytes := rawBuffer.Bytes()
	if rawSizeInKb := int64(len(rawBytes))/1024 + 1; rawSizeInKb > estimatedRawSizeInKb {
		return "", "", 0, fmt.Errorf("Bitcoin Cash raw size estimation error %d %d", rawSizeInKb, estimatedRawSizeInKb)
	}
	if estimatedRawSizeInKb > 100 {
		return "", "", 0, fmt.Errorf("Bitcoin Cash transaction size too large %d", estimatedRawSizeInKb)
	}
	return tx.TxHash().String(), hex.EncodeToString(rawBytes), feeToConsumed, nil
}

func LocalGenerateKey() (string, string, error) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	if err != nil {
		return "", "", err
	}
	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), seed)
	addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
	if err != nil {
		return "", "", err
	}
	private := hex.EncodeToString(privateKey.Serialize())
	public := addressPubKey.AddressPubKeyHash().EncodeAddress()
	return public, private, nil
}
