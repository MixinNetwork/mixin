package external

import (
	"fmt"

	bitcoinCash "mixin.one/blockchain/external/bitcoin-cash/api"
	bitcoin "mixin.one/blockchain/external/bitcoin/api"
	ethereumClassic "mixin.one/blockchain/external/ethereum-classic/api"
	ethereum "mixin.one/blockchain/external/ethereum/api"
	litecoin "mixin.one/blockchain/external/litecoin/api"
	ripple "mixin.one/blockchain/external/ripple/api"
	"mixin.one/number"
)

type RawTransaction struct {
	TransactionHash string
	RawTransaction  string
}

type UTXOTransaction struct {
	TransactionHash   string
	RawTransaction    string
	Input             number.Decimal
	Fee               number.Decimal
	ChangeIndex       int64
	bitcoinInputs     []*bitcoin.UTXO
	bitcoinCashInputs []*bitcoinCash.UTXO
	litecoinInputs    []*litecoin.UTXO
}

func SignRawTransaction(asset *Asset, receiver string, amount number.Decimal, gasPrice, gasLimit number.Decimal, privateKey string, nonce int) (*RawTransaction, error) {
	switch asset.ChainId {
	case EthereumChainId:
		ethAsset := &ethereum.Asset{Key: asset.ChainAssetKey, Precision: int32(asset.Precision)}
		transactionHash, rawTransaction, err := ethereum.LocalSignRawTransaction(ethAsset, receiver, amount, gasPrice, gasLimit, privateKey, uint64(nonce))
		if err != nil {
			return nil, err
		}
		return &RawTransaction{
			TransactionHash: transactionHash,
			RawTransaction:  rawTransaction,
		}, nil
	case EthereumClassicChainId:
		ethAsset := &ethereumClassic.Asset{Key: asset.ChainAssetKey, Precision: int32(asset.Precision)}
		transactionHash, rawTransaction, err := ethereumClassic.LocalSignRawTransaction(ethAsset, receiver, amount, gasPrice, gasLimit, privateKey, uint64(nonce))
		if err != nil {
			return nil, err
		}
		return &RawTransaction{
			TransactionHash: transactionHash,
			RawTransaction:  rawTransaction,
		}, nil
	case RippleChainId:
		rippleAsset := &ripple.Asset{Key: asset.ChainAssetKey, Precision: int32(asset.Precision)}
		transactionHash, rawTransaction, err := ripple.LocalSignRawTransaction(rippleAsset, receiver, amount, gasPrice, privateKey, uint32(nonce))
		if err != nil {
			return nil, err
		}
		return &RawTransaction{
			TransactionHash: transactionHash,
			RawTransaction:  rawTransaction,
		}, nil
	}
	return nil, fmt.Errorf("unsupported chain id %s", asset.ChainId)
}

type UTXOInput struct {
	TransactionHash string
	Index           uint32
	Amount          number.Decimal
	PrivateKey      string
}

func UTXOTransactionPrepare(chainId string, tx *UTXOTransaction, vin *UTXOInput, feePerKb number.Decimal) (*UTXOTransaction, error) {
	switch chainId {
	case BitcoinChainId:
		tx.bitcoinInputs = append(tx.bitcoinInputs, &bitcoin.UTXO{
			TransactionHash: vin.TransactionHash,
			Index:           vin.Index,
			Amount:          int64(vin.Amount.Mul(number.FromString("100000000")).Float64()),
			PrivateKey:      vin.PrivateKey,
		})
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		feeSatoshi := bitcoin.LocalEstimateTransactionFee(tx.bitcoinInputs, feePerKbSatoshi)
		tx.Fee = number.FromString(fmt.Sprint(feeSatoshi)).Mul(number.FromString("0.00000001"))
		tx.Input = tx.Input.Add(vin.Amount)
		return tx, nil
	case BitcoinCashChainId:
		tx.bitcoinCashInputs = append(tx.bitcoinCashInputs, &bitcoinCash.UTXO{
			TransactionHash: vin.TransactionHash,
			Index:           vin.Index,
			Amount:          int64(vin.Amount.Mul(number.FromString("100000000")).Float64()),
			PrivateKey:      vin.PrivateKey,
		})
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		feeSatoshi := bitcoinCash.LocalEstimateTransactionFee(tx.bitcoinCashInputs, feePerKbSatoshi)
		tx.Fee = number.FromString(fmt.Sprint(feeSatoshi)).Mul(number.FromString("0.00000001"))
		tx.Input = tx.Input.Add(vin.Amount)
		return tx, nil
	case LitecoinChainId:
		tx.litecoinInputs = append(tx.litecoinInputs, &litecoin.UTXO{
			TransactionHash: vin.TransactionHash,
			Index:           vin.Index,
			Amount:          int64(vin.Amount.Mul(number.FromString("100000000")).Float64()),
			PrivateKey:      vin.PrivateKey,
		})
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		feeSatoshi := litecoin.LocalEstimateTransactionFee(tx.litecoinInputs, feePerKbSatoshi)
		tx.Fee = number.FromString(fmt.Sprint(feeSatoshi)).Mul(number.FromString("0.00000001"))
		tx.Input = tx.Input.Add(vin.Amount)
		return tx, nil
	}
	return nil, fmt.Errorf("unsupported chain id %s", chainId)
}

func UTXOTransactionFinalize(chainId string, tx *UTXOTransaction, amount, feePerKb number.Decimal, receiverAddress, changeAddress string) (*UTXOTransaction, error) {
	switch chainId {
	case BitcoinChainId:
		amountSatoshi := int64(amount.Mul(number.FromString("100000000")).Float64())
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		transactionHash, rawTransaction, consumedFee, err := bitcoin.LocalSignRawTransaction(tx.bitcoinInputs, receiverAddress, amountSatoshi, feePerKbSatoshi, changeAddress)
		if err != nil {
			return nil, err
		}
		tx.TransactionHash = transactionHash
		tx.RawTransaction = rawTransaction
		tx.Fee = number.FromString(fmt.Sprint(consumedFee)).Mul(number.FromString("0.00000001"))
		tx.ChangeIndex = 1
		return tx, nil
	case BitcoinCashChainId:
		amountSatoshi := int64(amount.Mul(number.FromString("100000000")).Float64())
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		transactionHash, rawTransaction, consumedFee, err := bitcoinCash.LocalSignRawTransaction(tx.bitcoinCashInputs, receiverAddress, amountSatoshi, feePerKbSatoshi, changeAddress)
		if err != nil {
			return nil, err
		}
		tx.TransactionHash = transactionHash
		tx.RawTransaction = rawTransaction
		tx.Fee = number.FromString(fmt.Sprint(consumedFee)).Mul(number.FromString("0.00000001"))
		tx.ChangeIndex = 1
		return tx, nil
	case LitecoinChainId:
		amountSatoshi := int64(amount.Mul(number.FromString("100000000")).Float64())
		feePerKbSatoshi := int64(feePerKb.Mul(number.FromString("100000000")).Float64())
		transactionHash, rawTransaction, consumedFee, err := litecoin.LocalSignRawTransaction(tx.litecoinInputs, receiverAddress, amountSatoshi, feePerKbSatoshi, changeAddress)
		if err != nil {
			return nil, err
		}
		tx.TransactionHash = transactionHash
		tx.RawTransaction = rawTransaction
		tx.Fee = number.FromString(fmt.Sprint(consumedFee)).Mul(number.FromString("0.00000001"))
		tx.ChangeIndex = 1
		return tx, nil
	}
	return nil, fmt.Errorf("unsupported chain id %s", chainId)
}
