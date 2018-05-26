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
		return 100, nil
	case BitcoinChainId:
		return 6, nil
	case BitcoinCashChainId:
		return 24, nil
	case LitecoinChainId:
		return 24, nil
	case RippleChainId:
		return 1, nil
	case SiacoinChainId:
		return 12, nil
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

func ShiftTransactionState(chainId string, confirmations int64) string {
	threshold, err := ChainThreshold(chainId)
	if err != nil {
		return TransactionStatePending
	}
	switch chainId {
	case EthereumChainId:
		threshold = 6
	case EthereumClassicChainId:
		threshold = 6
	}
	if confirmations >= int64(threshold) {
		return TransactionStateConfirmed
	}
	return TransactionStatePending
}
