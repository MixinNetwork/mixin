package api

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
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

type Output struct {
	TransactionHash string
	RawTransaction  string
	Fee             int64
	OutputIndex     int64
	OutputHash      string
	ChangeIndex     int64
	ChangeHash      string
	ChangeAmount    int64
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

func LocalSignRawTransaction(inputs []*UTXO, output string, amount int64, feePerKb int64, changeAddress string) (*Output, error) {
	tx, inputAmount := wire.NewMsgTx(wire.TxVersion), int64(0)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.TransactionHash)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
	if err != nil {
		return nil, err
	}
	tx.AddTxOut(wire.NewTxOut(amount, pkScript))

	estimatedRawSizeInKb := int64(len(inputs))*160/1024 + 1
	feeToConsumed := feePerKb * estimatedRawSizeInKb
	changeAmount := inputAmount - feeToConsumed - amount
	if changeAmount < 0 {
		return nil, fmt.Errorf("insuficcient trasaction fee %d %d %d", inputAmount, feePerKb, estimatedRawSizeInKb)
	}
	if changeAmount > feePerKb {
		addressPubKeyHash, err := btcutil.DecodeAddress(changeAddress, &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return nil, err
		}
		tx.AddTxOut(wire.NewTxOut(changeAmount, pkScript))
	} else {
		feeToConsumed = inputAmount - amount
		changeAmount = 0
	}

	for idx, input := range inputs {
		privateKeyBytes, err := hex.DecodeString(input.PrivateKey)
		if err != nil {
			return nil, err
		}
		privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
		if err != nil {
			return nil, err
		}
		addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeCompressed(), &chaincfg.MainNetParams)
		if err != nil {
			return nil, err
		}
		addressPubKeyHash := addressPubKey.AddressPubKeyHash()
		pkScript, err := txscript.PayToAddrScript(addressPubKeyHash)
		if err != nil {
			return nil, err
		}
		sigScript, err := bchutil.SignatureScript(tx, idx, pkScript, txscript.SigHashAll, privateKey, true, input.Amount)
		if err != nil {
			return nil, err
		}
		tx.TxIn[idx].SignatureScript = sigScript
	}

	var rawBuffer bytes.Buffer
	err = tx.BtcEncode(&rawBuffer, wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return nil, err
	}
	rawBytes := rawBuffer.Bytes()
	if rawSizeInKb := int64(len(rawBytes))/1024 + 1; rawSizeInKb > estimatedRawSizeInKb {
		return nil, fmt.Errorf("Bitcoin Cash raw size estimation error %d %d", rawSizeInKb, estimatedRawSizeInKb)
	}
	if estimatedRawSizeInKb > 30 {
		return nil, fmt.Errorf("Bitcoin Cash transaction size too large %d", estimatedRawSizeInKb)
	}
	transactionHash := tx.TxHash().String()
	outputHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", transactionHash, 0)))
	changeHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", transactionHash, 1)))
	return &Output{
		TransactionHash: transactionHash,
		RawTransaction:  hex.EncodeToString(rawBytes),
		Fee:             feeToConsumed,
		OutputIndex:     0,
		OutputHash:      hex.EncodeToString(outputHash[:]),
		ChangeIndex:     1,
		ChangeHash:      hex.EncodeToString(changeHash[:]),
		ChangeAmount:    changeAmount,
	}, nil
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
