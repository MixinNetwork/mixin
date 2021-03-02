package kernel

import (
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) buildMintTransactionV1(timestamp uint64, validateOnly bool) *common.VersionedTransaction {
	batch, amount := node.checkMintPossibility(timestamp, validateOnly)
	if amount.Sign() <= 0 || batch <= 0 {
		return nil
	}

	if batch < MainnetMintWorkDistributionForkBatch {
		return node.legacyMintTransaction(timestamp, batch, amount)
	}

	accepted := node.NodesListWithoutState(timestamp, true)
	mints, err := node.distributeMintByWorks(accepted, amount, timestamp)
	if err != nil {
		logger.Printf("buildMintTransaction ERROR %s\n", err.Error())
		return nil
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddKernelNodeMintInput(uint64(batch), amount)
	script := common.NewThresholdScript(1)
	total := common.NewInteger(0)
	for _, m := range mints {
		in := fmt.Sprintf("MINTKERNELNODE%d", batch)
		si := crypto.NewHash([]byte(m.Signer.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&m.Payee}, script, m.Work, seed)
		total = total.Add(m.Work)
	}
	if total.Cmp(amount) > 0 {
		panic(fmt.Errorf("buildMintTransaction %s %s", amount, total))
	}

	if diff := amount.Sub(total); diff.Sign() > 0 {
		addr := common.NewAddressFromSeed(make([]byte, 64))
		script := common.NewThresholdScript(common.Operator64)
		in := fmt.Sprintf("MINTKERNELNODE%dDIFF", batch)
		si := crypto.NewHash([]byte(addr.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&addr}, script, diff, seed)
	}
	ver := tx.AsLatestVersion()
	ver.Version = 1
	return ver
}

func (node *Node) legacyMintTransaction(timestamp uint64, batch int, amount common.Integer) *common.VersionedTransaction {
	nodes := node.NodesListWithoutState(timestamp, true)
	sort.Slice(nodes, func(i, j int) bool {
		a := nodes[i].IdForNetwork
		b := nodes[j].IdForNetwork
		return a.String() < b.String()
	})

	per := amount.Div(len(nodes))
	diff := amount.Sub(per.Mul(len(nodes)))

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddKernelNodeMintInput(uint64(batch), amount)
	script := common.NewThresholdScript(1)
	for _, n := range nodes {
		in := fmt.Sprintf("MINTKERNELNODE%d", batch)
		si := crypto.NewHash([]byte(n.Signer.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&n.Payee}, script, per, seed)
	}

	if diff.Sign() > 0 {
		addr := common.NewAddressFromSeed(make([]byte, 64))
		script := common.NewThresholdScript(common.Operator64)
		in := fmt.Sprintf("MINTKERNELNODE%dDIFF", batch)
		si := crypto.NewHash([]byte(addr.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&addr}, script, diff, seed)
	}

	ver := tx.AsLatestVersion()
	ver.Version = 1
	return ver
}
