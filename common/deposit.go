package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/dogecoin"
	"github.com/MixinNetwork/mixin/domains/eos"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/MixinNetwork/mixin/domains/horizen"
	"github.com/MixinNetwork/mixin/domains/mobilecoin"
	"github.com/MixinNetwork/mixin/domains/monero"
	"github.com/MixinNetwork/mixin/domains/polkadot"
	"github.com/MixinNetwork/mixin/domains/siacoin"
	"github.com/MixinNetwork/mixin/domains/tron"
	"github.com/MixinNetwork/mixin/domains/zcash"
)

type DepositData struct {
	Chain           crypto.Hash `json:"chain"`
	AssetKey        string      `json:"asset"`
	TransactionHash string      `json:"transaction"`
	OutputIndex     uint64      `json:"index"`
	Amount          Integer     `json:"amount"`
}

func (d *DepositData) Asset() *Asset {
	return &Asset{
		ChainId:  d.Chain,
		AssetKey: d.AssetKey,
	}
}

func (d *DepositData) UniqueKey() crypto.Hash {
	index := fmt.Sprintf("%s:%d", d.TransactionHash, d.OutputIndex)
	return crypto.NewHash([]byte(index)).ForNetwork(d.Chain)
}

func (tx *Transaction) DepositData() *DepositData {
	if len(tx.Inputs) != 1 {
		return nil
	}
	return tx.Inputs[0].Deposit
}

func (tx *Transaction) verifyDepositFormat() error {
	deposit := tx.Inputs[0].Deposit
	if err := deposit.Asset().Verify(); err != nil {
		return fmt.Errorf("invalid asset data %s", err.Error())
	}
	if id := deposit.Asset().AssetId(); id != tx.Asset {
		return fmt.Errorf("invalid asset %s %s", tx.Asset, id)
	}
	if deposit.Amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount %s", deposit.Amount.String())
	}

	chainId := deposit.Asset().ChainId
	switch chainId {
	case ethereum.EthereumChainId:
		return ethereum.VerifyTransactionHash(deposit.TransactionHash)
	case bitcoin.BitcoinChainId:
		return bitcoin.VerifyTransactionHash(deposit.TransactionHash)
	case monero.MoneroChainId:
		return monero.VerifyTransactionHash(deposit.TransactionHash)
	case zcash.ZcashChainId:
		return zcash.VerifyTransactionHash(deposit.TransactionHash)
	case horizen.HorizenChainId:
		return horizen.VerifyTransactionHash(deposit.TransactionHash)
	case dogecoin.DogecoinChainId:
		return dogecoin.VerifyTransactionHash(deposit.TransactionHash)
	case siacoin.SiacoinChainId:
		return siacoin.VerifyTransactionHash(deposit.TransactionHash)
	case polkadot.PolkadotChainId:
		return polkadot.VerifyTransactionHash(deposit.TransactionHash)
	case eos.EOSChainId:
		return eos.VerifyTransactionHash(deposit.TransactionHash)
	case tron.TronChainId:
		return tron.VerifyTransactionHash(deposit.TransactionHash)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.VerifyTransactionHash(deposit.TransactionHash)
	}
	return fmt.Errorf("invalid deposit chain id %s", chainId)
}

func (tx *SignedTransaction) validateDeposit(store DataStore, msg []byte, payloadHash crypto.Hash) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for deposit", len(tx.Inputs))
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for deposit", len(tx.Outputs))
	}
	if tx.Outputs[0].Type != OutputTypeScript {
		return fmt.Errorf("invalid deposit output type %d", tx.Outputs[0].Type)
	}
	if len(tx.SignaturesMap) != 1 || len(tx.SignaturesMap[0]) != 1 {
		return fmt.Errorf("invalid signatures count %d for deposit", len(tx.SignaturesMap))
	}
	err := tx.verifyDepositFormat()
	if err != nil {
		return err
	}

	sig, valid := tx.SignaturesMap[0][0], false
	if sig == nil {
		return fmt.Errorf("invalid domain signature index for deposit")
	}
	for _, d := range store.ReadDomains() {
		if d.Account.PublicSpendKey.Verify(msg, *sig) {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("invalid domain signature for deposit")
	}

	return store.CheckDepositInput(tx.Inputs[0].Deposit, payloadHash)
}

func (tx *Transaction) AddDepositInput(data *DepositData) {
	tx.Inputs = append(tx.Inputs, &Input{
		Deposit: data,
	})
}
