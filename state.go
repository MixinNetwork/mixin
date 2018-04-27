package external

import "fmt"

const (
	TransactionReceiptSuccessful = "successful"
	TransactionReceiptFailed     = "failed"

	TransactionStatePending   = "pending"
	TransactionStateConfirmed = "confirmed"
	TransactionStateFailed    = "failed"
	UTXOStateUnspent          = "unspent"
	UTXOStateSpent            = "spent"
)

func ChainThreshold(chainId string) (int, error) {
	switch chainId {
	case EthereumChainId:
		return 36, nil
	case EthereumClassicChainId:
		return 72, nil
	case BitcoinChainId:
		return 6, nil
	case BitcoinCashChainId:
		return 12, nil
	case LitecoinChainId:
		return 12, nil
	case RippleChainId:
		return 1, nil
	}
	return 0, fmt.Errorf("unsupported chain id %s", chainId)
}

func TransactionState(chainId string, confirmations int64) string {
	threshold, err := ChainThreshold(chainId)
	if err != nil {
		return TransactionStatePending
	}
	if confirmations >= int64(threshold) {
		return TransactionStateConfirmed
	}
	return TransactionStatePending
}
