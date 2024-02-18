package server

import "github.com/MixinNetwork/mixin/storage"

func getCustodianHistory(store storage.Store, params []any) ([]map[string]any, error) {
	curs, err := store.ListCustodianUpdates()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, len(curs))
	for i, cur := range curs {
		item := map[string]any{
			"custodian":   cur.Custodian.String(),
			"transaction": cur.Transaction.String(),
			"timestamp":   cur.Timestamp,
		}
		result[i] = item
	}
	return result, nil
}
