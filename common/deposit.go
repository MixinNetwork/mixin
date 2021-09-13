package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/akash"
	"github.com/MixinNetwork/mixin/domains/algorand"
	"github.com/MixinNetwork/mixin/domains/arweave"
	"github.com/MixinNetwork/mixin/domains/avalanche"
	"github.com/MixinNetwork/mixin/domains/bch"
	"github.com/MixinNetwork/mixin/domains/binance"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/bsv"
	"github.com/MixinNetwork/mixin/domains/cosmos"
	"github.com/MixinNetwork/mixin/domains/dash"
	"github.com/MixinNetwork/mixin/domains/decred"
	"github.com/MixinNetwork/mixin/domains/dfinity"
	"github.com/MixinNetwork/mixin/domains/dogecoin"
	"github.com/MixinNetwork/mixin/domains/eos"
	"github.com/MixinNetwork/mixin/domains/etc"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/MixinNetwork/mixin/domains/filecoin"
	"github.com/MixinNetwork/mixin/domains/handshake"
	"github.com/MixinNetwork/mixin/domains/horizen"
	"github.com/MixinNetwork/mixin/domains/kusama"
	"github.com/MixinNetwork/mixin/domains/litecoin"
	"github.com/MixinNetwork/mixin/domains/mobilecoin"
	"github.com/MixinNetwork/mixin/domains/monero"
	"github.com/MixinNetwork/mixin/domains/namecoin"
	"github.com/MixinNetwork/mixin/domains/near"
	"github.com/MixinNetwork/mixin/domains/nervos"
	"github.com/MixinNetwork/mixin/domains/polkadot"
	"github.com/MixinNetwork/mixin/domains/ravencoin"
	"github.com/MixinNetwork/mixin/domains/ripple"
	"github.com/MixinNetwork/mixin/domains/siacoin"
	"github.com/MixinNetwork/mixin/domains/solana"
	"github.com/MixinNetwork/mixin/domains/stellar"
	"github.com/MixinNetwork/mixin/domains/tezos"
	"github.com/MixinNetwork/mixin/domains/tron"
	"github.com/MixinNetwork/mixin/domains/zcash"
)

type DepositData struct {
	Chain           crypto.Hash
	AssetKey        string
	TransactionHash string
	OutputIndex     uint64
	Amount          Integer
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
	case etc.EthereumClassicChainId:
		return etc.VerifyTransactionHash(deposit.TransactionHash)
	case bitcoin.BitcoinChainId:
		return bitcoin.VerifyTransactionHash(deposit.TransactionHash)
	case monero.MoneroChainId:
		return monero.VerifyTransactionHash(deposit.TransactionHash)
	case zcash.ZcashChainId:
		return zcash.VerifyTransactionHash(deposit.TransactionHash)
	case horizen.HorizenChainId:
		return horizen.VerifyTransactionHash(deposit.TransactionHash)
	case litecoin.LitecoinChainId:
		return litecoin.VerifyTransactionHash(deposit.TransactionHash)
	case dogecoin.DogecoinChainId:
		return dogecoin.VerifyTransactionHash(deposit.TransactionHash)
	case ravencoin.RavencoinChainId:
		return ravencoin.VerifyTransactionHash(deposit.TransactionHash)
	case namecoin.NamecoinChainId:
		return namecoin.VerifyTransactionHash(deposit.TransactionHash)
	case dash.DashChainId:
		return dash.VerifyTransactionHash(deposit.TransactionHash)
	case decred.DecredChainId:
		return decred.VerifyTransactionHash(deposit.TransactionHash)
	case bch.BitcoinCashChainId:
		return bch.VerifyTransactionHash(deposit.TransactionHash)
	case bsv.BitcoinSVChainId:
		return bsv.VerifyTransactionHash(deposit.TransactionHash)
	case handshake.HandshakenChainId:
		return handshake.VerifyTransactionHash(deposit.TransactionHash)
	case nervos.NervosChainId:
		return nervos.VerifyTransactionHash(deposit.TransactionHash)
	case siacoin.SiacoinChainId:
		return siacoin.VerifyTransactionHash(deposit.TransactionHash)
	case filecoin.FilecoinChainId:
		return filecoin.VerifyTransactionHash(deposit.TransactionHash)
	case solana.SolanaChainId:
		return solana.VerifyTransactionHash(deposit.TransactionHash)
	case near.NearChainId:
		return near.VerifyTransactionHash(deposit.TransactionHash)
	case polkadot.PolkadotChainId:
		return polkadot.VerifyTransactionHash(deposit.TransactionHash)
	case kusama.KusamaChainId:
		return kusama.VerifyTransactionHash(deposit.TransactionHash)
	case ripple.RippleChainId:
		return ripple.VerifyTransactionHash(deposit.TransactionHash)
	case stellar.StellarChainId:
		return stellar.VerifyTransactionHash(deposit.TransactionHash)
	case tezos.TezosChainId:
		return tezos.VerifyTransactionHash(deposit.TransactionHash)
	case eos.EOSChainId:
		return eos.VerifyTransactionHash(deposit.TransactionHash)
	case tron.TronChainId:
		return tron.VerifyTransactionHash(deposit.TransactionHash)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.VerifyTransactionHash(deposit.TransactionHash)
	case cosmos.CosmosChainId:
		return cosmos.VerifyTransactionHash(deposit.TransactionHash)
	case avalanche.AvalancheChainId:
		return avalanche.VerifyTransactionHash(deposit.TransactionHash)
	case binance.BinanceChainId:
		return binance.VerifyTransactionHash(deposit.TransactionHash)
	case akash.AkashChainId:
		return akash.VerifyTransactionHash(deposit.TransactionHash)
	case arweave.ArweaveChainId:
		return arweave.VerifyTransactionHash(deposit.TransactionHash)
	case dfinity.DfinityChainId:
		return dfinity.VerifyTransactionHash(deposit.TransactionHash)
	case algorand.AlgorandChainId:
		return algorand.VerifyTransactionHash(deposit.TransactionHash)
	}
	return fmt.Errorf("invalid deposit chain id %s", chainId)
}

func (tx *SignedTransaction) validateDeposit(store DataStore, msg []byte, payloadHash crypto.Hash, sigs []map[uint16]*crypto.Signature) error {
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for deposit", len(tx.Inputs))
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for deposit", len(tx.Outputs))
	}
	if tx.Outputs[0].Type != OutputTypeScript {
		return fmt.Errorf("invalid deposit output type %d", tx.Outputs[0].Type)
	}
	if len(sigs) != 1 || len(sigs[0]) != 1 {
		return fmt.Errorf("invalid signatures count %d for deposit", len(sigs))
	}
	err := tx.verifyDepositFormat()
	if err != nil {
		return err
	}

	sig, valid := sigs[0][0], false
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
