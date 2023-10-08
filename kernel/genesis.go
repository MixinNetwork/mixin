package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	MinimumNodeCount = 7
)

type Genesis struct {
	Epoch int64 `json:"epoch"`
	Nodes []*struct {
		Signer  common.Address `json:"signer"`
		Payee   common.Address `json:"payee"`
		Balance common.Integer `json:"balance"`
	} `json:"nodes"`
	Custodian common.Address `json:"custodian"`
}

func (node *Node) LoadGenesis(configDir string) error {
	gns, err := readGenesis(configDir + "/genesis.json")
	if err != nil {
		return err
	}

	data, err := json.Marshal(gns)
	if err != nil {
		return err
	}
	node.Epoch = uint64(time.Unix(gns.Epoch, 0).UnixNano())
	node.networkId = crypto.Blake3Hash(data)
	node.IdForNetwork = node.Signer.Hash().ForNetwork(node.networkId)
	for _, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(node.networkId)
		node.genesisNodesMap[id] = true
		node.genesisNodes = append(node.genesisNodes, id)
	}

	rounds, snapshots, transactions, err := buildGenesisSnapshots(node.networkId, node.Epoch, gns)
	if err != nil {
		return err
	}

	loaded, err := node.persistStore.CheckGenesisLoad(snapshots)
	if err != nil || loaded {
		return err
	}

	return node.persistStore.LoadGenesis(rounds, snapshots, transactions)
}

func buildGenesisSnapshots(networkId crypto.Hash, epoch uint64, gns *Genesis) ([]*common.Round, []*common.SnapshotWithTopologicalOrder, []*common.VersionedTransaction, error) {
	var snapshots []*common.SnapshotWithTopologicalOrder
	var transactions []*common.VersionedTransaction
	cacheRounds := make(map[crypto.Hash]*CacheRound)
	for i, in := range gns.Nodes {
		si := crypto.Blake3Hash([]byte(in.Signer.String() + "NODEACCEPT"))
		seed := append(si[:], si[:]...)
		script := common.NewThresholdScript(uint8(len(gns.Nodes)*2/3 + 1))
		accounts := []*common.Address{}
		for _, d := range gns.Nodes {
			accounts = append(accounts, &d.Signer)
		}

		tx := common.NewTransactionV5(common.XINAssetId)
		tx.Inputs = []*common.Input{{Genesis: networkId[:]}}
		tx.AddOutputWithType(common.OutputTypeNodeAccept, accounts, script, pledgeAmount(0), seed)
		tx.Extra = append(in.Signer.PublicSpendKey[:], in.Payee.PublicSpendKey[:]...)

		nodeId := in.Signer.Hash().ForNetwork(networkId)
		snapshot := &common.Snapshot{
			Version:     common.SnapshotVersionCommonEncoding,
			NodeId:      nodeId,
			RoundNumber: 0,
			Timestamp:   epoch,
		}
		signed := tx.AsVersioned()
		snapshot.AddSoleTransaction(signed.PayloadHash())
		snapshot.Hash = snapshot.PayloadHash()
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         snapshot,
			TopologicalOrder: uint64(i),
		}
		snapshots = append(snapshots, topo)
		transactions = append(transactions, signed)
		cacheRounds[snapshot.NodeId] = &CacheRound{
			NodeId:    snapshot.NodeId,
			Number:    0,
			Snapshots: []*common.Snapshot{snapshot},
		}
	}

	topo, signed := buildCustodianSnapshot(networkId, epoch, gns.Custodian, gns)
	snapshots = append(snapshots, topo)
	transactions = append(transactions, signed)
	snap := topo.Snapshot
	snap.Hash = snap.PayloadHash()
	cacheRounds[topo.NodeId].Snapshots = append(cacheRounds[topo.NodeId].Snapshots, snap)

	rounds := make([]*common.Round, 0)
	for i, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(networkId)
		external := gns.Nodes[0].Signer.Hash().ForNetwork(networkId)
		if i != len(gns.Nodes)-1 {
			external = gns.Nodes[i+1].Signer.Hash().ForNetwork(networkId)
		}
		selfFinal := cacheRounds[id].asFinal()
		externalFinal := cacheRounds[external].asFinal()
		rounds = append(rounds, &common.Round{
			Hash:      selfFinal.Hash,
			NodeId:    selfFinal.NodeId,
			Number:    selfFinal.Number,
			Timestamp: selfFinal.Start,
		})
		rounds = append(rounds, &common.Round{
			Hash:   selfFinal.NodeId,
			NodeId: selfFinal.NodeId,
			Number: selfFinal.Number + 1,
			References: &common.RoundLink{
				Self:     selfFinal.Hash,
				External: externalFinal.Hash,
			},
		})
	}

	return rounds, snapshots, transactions, nil
}

func buildCustodianSnapshot(networkId crypto.Hash, epoch uint64, domain common.Address, gns *Genesis) (*common.SnapshotWithTopologicalOrder, *common.VersionedTransaction) {
	panic(0)
}

func readGenesis(path string) (*Genesis, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var gns Genesis
	err = json.Unmarshal(f, &gns)
	if err != nil {
		return nil, err
	}
	if len(gns.Nodes) < MinimumNodeCount {
		return nil, fmt.Errorf("invalid genesis inputs number %d/%d", len(gns.Nodes), MinimumNodeCount)
	}

	inputsFilter := make(map[string]bool)
	for _, in := range gns.Nodes {
		_, err := common.NewAddressFromString(in.Signer.String())
		if err != nil {
			return nil, err
		}
		if in.Balance.Cmp(pledgeAmount(0)) != 0 {
			return nil, fmt.Errorf("invalid genesis node input amount %s", in.Balance.String())
		}
		if inputsFilter[in.Signer.String()] {
			return nil, fmt.Errorf("duplicated genesis node input %s", in.Signer.String())
		}
		privateView := in.Signer.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Signer.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s",
				privateView.Public().String(), in.Signer.PublicViewKey.String())
		}
		privateView = in.Payee.PublicSpendKey.DeterministicHashDerive()
		if privateView.Public() != in.Payee.PublicViewKey {
			return nil, fmt.Errorf("invalid node key format %s %s",
				privateView.Public().String(), in.Payee.PublicViewKey.String())
		}
	}

	panic("check gns.Custodian")
	return &gns, nil
}
