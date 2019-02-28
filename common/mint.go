package common

type MintData struct {
	Batch  uint64  `json:"batch"`
	Amount Integer `json:"amount"`
}

type MintDistribution struct {
	Batch  uint64  `json:"batch"`
	Amount Integer `json:"amount"`
}

func (tx *SignedTransaction) validateMintInput(store DataStore) error {
	return nil
}
