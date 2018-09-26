package kernel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

const (
	MinimumNodeCount = 7
	PledgeAmount     = 10000
)

type Genesis []struct {
	Address *common.Address `json:"address"`
	Balance common.Integer  `json:"balance"`
	Mask    string          `json:"mask"`
}

func loadGenesis(store storage.Store, configDir string) (string, error) {
	const stateKeyNetwork = "network"

	gns, err := readGenesis(configDir + "/genesis.json")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(gns)
	if err != nil {
		return "", err
	}
	networkId := crypto.NewHash(data)

	var network struct {
		Id crypto.Hash
	}
	found, err := store.StateGet(stateKeyNetwork, &network)
	if err != nil {
		return "", err
	}
	if network.Id.String() == networkId.String() {
		return networkId.String(), nil
	}
	if found {
		return "", fmt.Errorf("invalid genesis for network %s", network.Id.String())
	}

	var snapshots []*common.Snapshot
	for i, in := range gns {
		r := crypto.NewKeyFromSeed([]byte(in.Mask))
		R := r.Public()
		var keys []crypto.Key
		for _, d := range gns {
			key := crypto.DeriveGhostPublicKey(&r, &d.Address.PublicViewKey, &d.Address.PublicSpendKey)
			keys = append(keys, *key)
		}

		tx := common.Transaction{
			Version: common.TxVersion,
			Asset:   common.XINAssetId,
			Inputs: []*common.Input{
				{
					Hash:  crypto.Hash{},
					Index: i,
				},
			},
			Outputs: []*common.Output{
				{
					Type:   common.OutputTypePledge,
					Script: common.Script([]uint8{common.OperatorCmp, common.OperatorSum, MinimumNodeCount*2/3 + 1}),
					Amount: common.NewInteger(PledgeAmount),
					Keys:   keys,
					Mask:   R,
				},
			},
		}

		remaining := in.Balance.Sub(common.NewInteger(PledgeAmount))
		if remaining.Cmp(common.NewInteger(0)) > 0 {
			r := crypto.NewKeyFromSeed(append(r[:], r[:]...))
			R := r.Public()
			key := crypto.DeriveGhostPublicKey(&r, &in.Address.PublicViewKey, &in.Address.PublicSpendKey)
			tx.Outputs = append(tx.Outputs, &common.Output{
				Type:   common.OutputTypeScript,
				Script: common.Script([]uint8{common.OperatorCmp, common.OperatorSum, 1}),
				Amount: remaining,
				Keys:   []crypto.Key{*key},
				Mask:   R,
			})
		}

		signed := &common.SignedTransaction{Transaction: tx}
		nodeId := in.Address.Hash()
		nodeId = crypto.NewHash(append(networkId[:], nodeId[:]...))
		snapshot := &common.Snapshot{
			NodeId:      nodeId,
			Transaction: signed,
			RoundNumber: 0,
			Timestamp:   0,
		}
		snapshots = append(snapshots, snapshot)
	}
	err = store.SnapshotsLoadGenesis(snapshots)
	if err != nil {
		return "", err
	}

	network.Id = networkId
	return networkId.String(), store.StateSet(stateKeyNetwork, network)
}

func readGenesis(path string) (Genesis, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gns Genesis
	err = json.Unmarshal(f, &gns)
	if err != nil {
		return nil, err
	}
	if len(gns) != MinimumNodeCount {
		return nil, fmt.Errorf("invalid genesis inputs number %d/%d", len(gns), MinimumNodeCount)
	}

	inputsFilter := make(map[string]bool)
	for _, in := range gns {
		_, err := common.NewAddressFromString(in.Address.String())
		if err != nil {
			return nil, err
		}
		if in.Balance.Cmp(common.NewInteger(PledgeAmount)) < 0 {
			return nil, fmt.Errorf("invalid genesis input amount %s", in.Balance.String())
		}
		if inputsFilter[in.Address.String()] {
			return nil, fmt.Errorf("duplicated genesis inputs %s", in.Address.String())
		}
	}
	return gns, nil
}
