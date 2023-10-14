package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/urfave/cli/v2"
)

func createAddressCmd(c *cli.Context) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	addr := common.NewAddressFromSeed(seed)
	if view := c.String("view"); len(view) > 0 {
		key, err := hex.DecodeString(view)
		if err != nil {
			return err
		}
		copy(addr.PrivateViewKey[:], key)
		addr.PublicViewKey = addr.PrivateViewKey.Public()
	}
	if spend := c.String("spend"); len(spend) > 0 {
		key, err := hex.DecodeString(spend)
		if err != nil {
			return err
		}
		copy(addr.PrivateSpendKey[:], key)
		addr.PublicSpendKey = addr.PrivateSpendKey.Public()
	}
	if c.Bool("public") {
		addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
		addr.PublicViewKey = addr.PrivateViewKey.Public()
	}
	fmt.Printf("address:\t%s\n", addr.String())
	fmt.Printf("view key:\t%s\n", addr.PrivateViewKey.String())
	fmt.Printf("spend key:\t%s\n", addr.PrivateSpendKey.String())
	return nil
}

func decodeAddressCmd(c *cli.Context) error {
	addr, err := common.NewAddressFromString(c.String("address"))
	if err != nil {
		return err
	}
	fmt.Printf("public view key:\t%s\n", addr.PublicViewKey.String())
	fmt.Printf("public spend key:\t%s\n", addr.PublicSpendKey.String())
	fmt.Printf("spend derive private:\t%s\n", addr.PublicSpendKey.DeterministicHashDerive())
	fmt.Printf("spend derive public:\t%s\n", addr.PublicSpendKey.DeterministicHashDerive().Public())
	return nil
}

func decodeSignatureCmd(c *cli.Context) error {
	var s struct{ S crypto.CosiSignature }
	in := fmt.Sprintf(`{"S":"%s"}`, c.String("signature"))
	err := json.Unmarshal([]byte(in), &s)
	if err != nil {
		return err
	}
	fmt.Printf("signers:\t%v\n", s.S.Keys())
	fmt.Printf("threshold:\t%d\n", len(s.S.Keys()))
	return nil
}

func decryptGhostCmd(c *cli.Context) error {
	view, err := crypto.KeyFromString(c.String("view"))
	if err != nil {
		return err
	}
	key, err := crypto.KeyFromString(c.String("key"))
	if err != nil {
		return err
	}
	mask, err := crypto.KeyFromString(c.String("mask"))
	if err != nil {
		return err
	}

	spend := crypto.ViewGhostOutputKey(&key, &view, &mask, c.Uint64("index"))
	addr := common.Address{
		PublicViewKey:  view.Public(),
		PublicSpendKey: *spend,
	}
	fmt.Print(addr.String())
	return nil
}

func updateHeadReference(c *cli.Context) error {
	custom, err := config.Initialize(c.String("dir") + "/config.toml")
	if err != nil {
		return err
	}
	store, err := storage.NewBadgerStore(custom, c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()
	node, err := crypto.HashFromString(c.String("id"))
	if err != nil {
		return err
	}
	round, err := store.ReadRound(node)
	if err != nil {
		return err
	}
	if round == nil {
		return errors.New("node not found")
	}
	fmt.Printf("node: %s round: %d self: %s external: %s\n", round.NodeId.String(), round.Number, round.References.Self.String(), round.References.External.String())
	if round.Number != c.Uint64("round") {
		return fmt.Errorf("round number not match %d", round.Number)
	}
	external, err := hex.DecodeString(c.String("external"))
	if err != nil {
		return err
	}
	copy(round.References.External[:], external)
	return store.UpdateEmptyHeadRound(round.NodeId, round.Number, round.References)
}

func removeGraphEntries(c *cli.Context) error {
	custom, err := config.Initialize(c.String("dir") + "/config.toml")
	if err != nil {
		return err
	}
	store, err := storage.NewBadgerStore(custom, c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()
	removed, err := store.RemoveGraphEntries(c.String("prefix"))
	fmt.Printf("removed %d entries with %v\n", removed, err)
	return err
}

func validateGraphEntries(c *cli.Context) error {
	custom, err := config.Initialize(c.String("dir") + "/config.toml")
	if err != nil {
		return err
	}

	f, err := os.ReadFile(c.String("dir") + "/genesis.json")
	if err != nil {
		return err
	}
	var gns kernel.Genesis
	err = json.Unmarshal(f, &gns)
	if err != nil {
		return err
	}
	data, err := json.Marshal(gns)
	if err != nil {
		return err
	}
	networkId := crypto.Blake3Hash(data)

	store, err := storage.NewBadgerStore(custom, c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()

	total, invalid, err := store.ValidateGraphEntries(networkId, c.Uint64("depth"))
	if err != nil {
		return err
	}
	fmt.Printf("invalid entries: %d/%d\n", invalid, total)
	return nil
}

func decodeTransactionCmd(c *cli.Context) error {
	raw, err := hex.DecodeString(c.String("raw"))
	if err != nil {
		return err
	}
	ver, err := common.UnmarshalVersionedTransaction(raw)
	if err != nil {
		return err
	}
	m := transactionToMap(ver)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func buildRawTransactionCmd(c *cli.Context) error {
	seed, err := hex.DecodeString(c.String("seed"))
	if err != nil {
		return err
	}
	if len(seed) != 64 {
		seed = make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return err
		}
	}

	viewKey, err := crypto.KeyFromString(c.String("view"))
	if err != nil {
		return err
	}
	spendKey, err := crypto.KeyFromString(c.String("spend"))
	if err != nil {
		return err
	}
	account := common.Address{
		PrivateViewKey:  viewKey,
		PrivateSpendKey: spendKey,
		PublicViewKey:   viewKey.Public(),
		PublicSpendKey:  spendKey.Public(),
	}

	asset, err := crypto.HashFromString(c.String("asset"))
	if err != nil {
		return err
	}

	extra, err := hex.DecodeString(c.String("extra"))
	if err != nil {
		return err
	}

	inputs := make([]map[string]any, 0)
	for _, in := range strings.Split(c.String("inputs"), ",") {
		parts := strings.Split(in, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid input %s", in)
		}
		hash, err := crypto.HashFromString(parts[0])
		if err != nil {
			return err
		}
		index, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}
		inputs = append(inputs, map[string]any{
			"hash":  hash,
			"index": int(index),
		})
	}

	outputs := make([]map[string]any, 0)
	for _, out := range strings.Split(c.String("outputs"), ",") {
		parts := strings.Split(out, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid output %s", out)
		}
		addr, err := common.NewAddressFromString(parts[0])
		if err != nil {
			return err
		}
		amount := common.NewIntegerFromString(parts[1])
		if amount.Sign() == 0 {
			return fmt.Errorf("invalid output %s", out)
		}
		outputs = append(outputs, map[string]any{
			"accounts": []*common.Address{&addr},
			"amount":   amount,
		})
	}

	var raw signerInput
	raw.Node = c.String("node")
	isb, _ := json.Marshal(map[string]any{"inputs": inputs})
	json.Unmarshal(isb, &raw)

	tx := common.NewTransactionV5(asset)
	for _, in := range inputs {
		tx.AddInput(in["hash"].(crypto.Hash), in["index"].(int))
	}
	for _, out := range outputs {
		tx.AddScriptOutput(out["accounts"].([]*common.Address), common.NewThresholdScript(1), out["amount"].(common.Integer), seed)
	}
	tx.Extra = extra

	signed := tx.AsVersioned()
	for i := range tx.Inputs {
		err = signed.SignInput(raw, i, []*common.Address{&account})
		if err != nil {
			return err
		}
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return nil
}

func signTransactionCmd(c *cli.Context) error {
	var raw signerInput
	err := json.Unmarshal([]byte(c.String("raw")), &raw)
	if err != nil {
		return err
	}
	raw.Node = c.String("node")

	seed, err := hex.DecodeString(c.String("seed"))
	if err != nil {
		return err
	}
	if len(seed) != 64 {
		seed = make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return err
		}
	}

	tx := common.NewTransactionV5(raw.Asset)
	for _, in := range raw.Inputs {
		if d := in.Deposit; d != nil {
			tx.AddDepositInput(&common.DepositData{
				Chain:       d.Chain,
				AssetKey:    d.AssetKey,
				Transaction: d.TransactionHash,
				Index:       d.OutputIndex,
				Amount:      d.Amount,
			})
		} else {
			tx.AddInput(in.Hash, in.Index)
		}
	}

	for _, out := range raw.Outputs {
		if out.Mask.HasValue() {
			tx.Outputs = append(tx.Outputs, &common.Output{
				Type:   out.Type,
				Amount: out.Amount,
				Keys:   out.Keys,
				Script: out.Script,
				Mask:   out.Mask,
			})
		} else {
			hash := crypto.Blake3Hash(seed)
			seed = append(hash[:], hash[:]...)
			tx.AddOutputWithType(out.Type, out.Accounts, out.Script, out.Amount, seed)
		}
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		return err
	}
	tx.Extra = extra

	keys := c.StringSlice("key")
	var accounts []*common.Address
	for _, s := range keys {
		key, err := hex.DecodeString(s)
		if err != nil {
			return err
		}
		if len(key) != 64 {
			return fmt.Errorf("invalid key length %d", len(key))
		}
		var account common.Address
		copy(account.PrivateViewKey[:], key[:32])
		copy(account.PrivateSpendKey[:], key[32:])
		accounts = append(accounts, &account)
	}

	signed := tx.AsVersioned()
	for i := range signed.Inputs {
		err := signed.SignInput(raw, i, accounts)
		if err != nil {
			return err
		}
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return nil
}

func sendTransactionCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "sendrawtransaction", []any{
		c.String("raw"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func pledgeNodeCmd(c *cli.Context) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	viewKey, err := crypto.KeyFromString(c.String("view"))
	if err != nil {
		return err
	}
	spendKey, err := crypto.KeyFromString(c.String("spend"))
	if err != nil {
		return err
	}
	account := common.Address{
		PrivateViewKey:  viewKey,
		PrivateSpendKey: spendKey,
		PublicViewKey:   viewKey.Public(),
		PublicSpendKey:  spendKey.Public(),
	}

	signer, err := common.NewAddressFromString(c.String("signer"))
	if err != nil {
		return err
	}
	payee, err := common.NewAddressFromString(c.String("payee"))
	if err != nil {
		return err
	}

	var raw signerInput
	input, err := crypto.HashFromString(c.String("input"))
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(fmt.Sprintf(`{"inputs":[{"hash":"%s","index":0}]}`, input.String())), &raw)
	if err != nil {
		return err
	}
	raw.Node = c.String("node")

	amount := common.NewIntegerFromString(c.String("amount"))

	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddInput(input, 0)
	tx.AddOutputWithType(common.OutputTypeNodePledge, nil, common.Script{}, amount, seed)
	tx.Extra = append(signer.PublicSpendKey[:], payee.PublicSpendKey[:]...)

	signed := tx.AsVersioned()
	err = signed.SignInput(raw, 0, []*common.Address{&account})
	if err != nil {
		return err
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return nil
}

func cancelNodeCmd(c *cli.Context) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	viewKey, err := crypto.KeyFromString(c.String("view"))
	if err != nil {
		return err
	}
	spendKey, err := crypto.KeyFromString(c.String("spend"))
	if err != nil {
		return err
	}
	receiver, err := common.NewAddressFromString(c.String("receiver"))
	if err != nil {
		return err
	}
	account := common.Address{
		PrivateViewKey:  viewKey,
		PrivateSpendKey: spendKey,
		PublicViewKey:   viewKey.Public(),
		PublicSpendKey:  spendKey.Public(),
	}
	if account.String() != receiver.String() {
		return fmt.Errorf("invalid key and receiver %s %s", account, receiver)
	}

	b, err := hex.DecodeString(c.String("pledge"))
	if err != nil {
		return err
	}
	pledge, err := common.UnmarshalVersionedTransaction(b)
	if err != nil {
		return err
	}
	if pledge.TransactionType() != common.TransactionTypeNodePledge {
		return fmt.Errorf("invalid pledge transaction type %d", pledge.TransactionType())
	}

	b, err = hex.DecodeString(c.String("source"))
	if err != nil {
		return err
	}
	source, err := common.UnmarshalVersionedTransaction(b)
	if err != nil {
		return err
	}
	if source.TransactionType() != common.TransactionTypeScript {
		return fmt.Errorf("invalid source transaction type %d", source.TransactionType())
	}

	if source.PayloadHash() != pledge.Inputs[0].Hash {
		return fmt.Errorf("invalid source transaction hash %s %s", source.PayloadHash(), pledge.Inputs[0].Hash)
	}
	if len(source.Outputs) != 1 || len(source.Outputs[0].Keys) != 1 {
		return fmt.Errorf("invalid source transaction outputs %d %d", len(source.Outputs), len(source.Outputs[0].Keys))
	}
	pig := crypto.ViewGhostOutputKey(source.Outputs[0].Keys[0], &viewKey, &source.Outputs[0].Mask, 0)
	if pig.String() != receiver.PublicSpendKey.String() {
		return fmt.Errorf("invalid source and receiver %s %s", pig.String(), receiver.PublicSpendKey)
	}

	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddInput(pledge.PayloadHash(), 0)
	tx.AddOutputWithType(common.OutputTypeNodeCancel, nil, common.Script{}, pledge.Outputs[0].Amount.Div(100), seed)
	tx.AddScriptOutput([]*common.Address{&receiver}, common.NewThresholdScript(1), pledge.Outputs[0].Amount.Sub(tx.Outputs[0].Amount), seed)
	tx.Extra = append(pledge.Extra, viewKey[:]...)
	utxo := &common.UTXO{
		Input: common.Input{
			Hash:  pledge.PayloadHash(),
			Index: 0,
		},
		Output: common.Output{
			Type: common.OutputTypeNodePledge,
			Keys: source.Outputs[0].Keys,
			Mask: source.Outputs[0].Mask,
		},
	}
	signed := tx.AsVersioned()
	err = signed.SignUTXO(utxo, []*common.Address{&account})
	if err != nil {
		return err
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return nil
}

func decodePledgeNodeCmd(c *cli.Context) error {
	b, err := hex.DecodeString(c.String("raw"))
	if err != nil {
		return err
	}
	pledge, err := common.UnmarshalVersionedTransaction(b)
	if err != nil {
		return err
	}
	if len(pledge.Extra) != len(crypto.Key{})*2 {
		return fmt.Errorf("invalid extra %s", hex.EncodeToString(pledge.Extra))
	}
	signerPublicSpend, err := crypto.KeyFromString(hex.EncodeToString(pledge.Extra[:32]))
	if err != nil {
		return err
	}
	payeePublicSpend, err := crypto.KeyFromString(hex.EncodeToString(pledge.Extra[32:]))
	if err != nil {
		return err
	}
	signer := common.Address{
		PublicSpendKey: signerPublicSpend,
		PublicViewKey:  signerPublicSpend.DeterministicHashDerive().Public(),
	}
	payee := common.Address{
		PublicSpendKey: payeePublicSpend,
		PublicViewKey:  payeePublicSpend.DeterministicHashDerive().Public(),
	}
	fmt.Printf("signer: %s\n", signer)
	fmt.Printf("payee: %s\n", payee)
	return nil
}

func encodeCustodianExtraCmd(c *cli.Context) error {
	signerSpend, err := crypto.KeyFromString(c.String("signer"))
	if err != nil {
		return err
	}
	payeeSpend, err := crypto.KeyFromString(c.String("payee"))
	if err != nil {
		return err
	}
	custodianSpend, err := crypto.KeyFromString(c.String("custodian"))
	if err != nil {
		return err
	}
	networkId, err := crypto.HashFromString(c.String("network"))
	if err != nil {
		return err
	}

	custodian := &common.Address{
		PublicSpendKey: custodianSpend.Public(),
		PublicViewKey:  custodianSpend.Public().DeterministicHashDerive().Public(),
	}
	payee := &common.Address{
		PublicSpendKey: payeeSpend.Public(),
		PublicViewKey:  payeeSpend.Public().DeterministicHashDerive().Public(),
	}
	extra := common.EncodeCustodianNode(custodian, payee, &signerSpend, &payeeSpend, &custodianSpend, networkId)
	fmt.Printf("HEX: %x\n", extra)
	fmt.Printf("BASE64: %s\n", base64.RawURLEncoding.EncodeToString(extra))
	return nil
}

func getRoundLinkCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getroundlink", []any{
		c.String("from"),
		c.String("to"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getRoundByNumberCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getroundbynumber", []any{
		c.String("id"),
		c.Uint64("number"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getRoundByHashCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getroundbyhash", []any{
		c.String("hash"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listSnapshotsCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listsnapshots", []any{
		c.Uint64("since"),
		c.Uint64("count"),
		c.Bool("sig"),
		c.Bool("tx"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getSnapshotCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getsnapshot", []any{
		c.String("hash"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getTransactionCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "gettransaction", []any{
		c.String("hash"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getCacheTransactionCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getcachetransaction", []any{
		c.String("hash"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getUTXOCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getutxo", []any{
		c.String("hash"),
		c.Uint64("index"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getKeyCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getkey", []any{
		c.String("key"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listCustodianUpdatesCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listcustodianupdates", []any{}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listMintWorksCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listmintworks", []any{
		c.Uint64("since"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listMintDistributionsCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listmintdistributions", []any{
		c.Uint64("since"),
		c.Uint64("count"),
		c.Bool("tx"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listAllNodesCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listallnodes", []any{
		c.Uint64("threshold"),
		c.Bool("state"),
	}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getInfoCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getinfo", []any{}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listPeersCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listpeers", []any{}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func dumpGraphHeadCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "dumpgraphhead", []any{}, c.Bool("time"))
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func setupTestNetCmd(c *cli.Context) error {
	var signers, payees []common.Address

	randomPubAccount := func() common.Address {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			panic(err)
		}
		account := common.NewAddressFromSeed(seed)
		account.PrivateViewKey = account.PublicSpendKey.DeterministicHashDerive()
		account.PublicViewKey = account.PrivateViewKey.Public()
		return account
	}
	for i := 0; i < 7; i++ {
		signers = append(signers, randomPubAccount())
		payees = append(payees, randomPubAccount())
	}

	inputs := make([]map[string]string, 0)
	for i := range signers {
		inputs = append(inputs, map[string]string{
			"signer":  signers[i].String(),
			"payee":   payees[i].String(),
			"balance": "10000",
		})
	}
	genesis := map[string]any{
		"epoch": time.Now().Unix(),
		"nodes": inputs,
		"domains": []map[string]string{
			{
				"signer":  signers[0].String(),
				"balance": "50000",
			},
		},
	}
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(genesisData))

	peers := make([]string, len(signers))
	for i := range signers {
		peers[i] = fmt.Sprintf("127.0.0.1:700%d", i+1)
	}
	peersList := `"` + strings.Join(peers, `","`) + `"`
	fmt.Println(peers)

	for i, a := range signers {
		dir := fmt.Sprintf("/tmp/mixin-700%d", i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}

		var configData = []byte(fmt.Sprintf(`
[node]
signer-key = "%s"
consensus-only = true
memory-cache-size = 128
cache-ttl = 3600
ring-cache-size = 4096
ring-final-size = 16384
[network]
listener = "%s"
peers = [%s]
`, a.PrivateSpendKey.String(), peers[i], peersList))

		err = os.WriteFile(dir+"/config.toml", configData, 0644)
		if err != nil {
			return err
		}
		err = os.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

var httpClient *http.Client

func callRPC(node, method string, params []any, pt bool) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	body, err := json.Marshal(map[string]any{
		"method": method,
		"params": params,
	})
	if err != nil {
		panic(err)
	}

	endpoint := "http://" + node
	if strings.HasPrefix(node, "http") {
		endpoint = node
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Runtime string `json:"runtime"`
		Data    any    `json:"data"`
		Error   any    `json:"error"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	err = dec.Decode(&result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("ERROR %s", result.Error)
	}

	if len(result.Runtime) > 0 && pt {
		fmt.Printf("RUNTIME: %s\n\n", result.Runtime)
	}
	return json.Marshal(result.Data)
}

type signerInput struct {
	Inputs []struct {
		Hash    crypto.Hash `json:"hash"`
		Index   int         `json:"index"`
		Deposit *struct {
			Chain           crypto.Hash    `json:"chain"`
			AssetKey        string         `json:"asset"`
			TransactionHash string         `json:"transaction"`
			OutputIndex     uint64         `json:"index"`
			Amount          common.Integer `json:"amount"`
		} `json:"deposit,omitempty"`
		Keys []*crypto.Key `json:"keys"`
		Mask crypto.Key    `json:"mask"`
	} `json:"inputs"`
	Outputs []struct {
		Type     uint8             `json:"type"`
		Mask     crypto.Key        `json:"mask"`
		Keys     []*crypto.Key     `json:"keys"`
		Amount   common.Integer    `json:"amount"`
		Script   common.Script     `json:"script"`
		Accounts []*common.Address `json:"accounts"`
	}
	Asset crypto.Hash `json:"asset"`
	Extra string      `json:"extra"`
	Node  string      `json:"-"`
}

func (raw signerInput) ReadUTXOKeys(hash crypto.Hash, index int) (*common.UTXOKeys, error) {
	utxo := &common.UTXOKeys{}

	for _, in := range raw.Inputs {
		if in.Hash == hash && in.Index == index && len(in.Keys) > 0 {
			utxo.Keys = in.Keys
			utxo.Mask = in.Mask
			return utxo, nil
		}
	}

	data, err := callRPC(raw.Node, "getutxo", []any{hash.String(), index}, false)
	if err != nil {
		return nil, err
	}
	var out common.UTXOWithLock
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	if out.Amount.Sign() == 0 {
		return nil, fmt.Errorf("invalid input %s#%d", hash.String(), index)
	}
	utxo.Keys = out.Keys
	utxo.Mask = out.Mask
	return utxo, nil
}

func (raw signerInput) CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	return nil
}

func transactionToMap(tx *common.VersionedTransaction) map[string]any {
	var inputs []map[string]any
	for _, in := range tx.Inputs {
		if in.Hash.HasValue() {
			inputs = append(inputs, map[string]any{
				"hash":  in.Hash,
				"index": in.Index,
			})
		} else if len(in.Genesis) > 0 {
			inputs = append(inputs, map[string]any{
				"genesis": hex.EncodeToString(in.Genesis),
			})
		} else if in.Deposit != nil {
			inputs = append(inputs, map[string]any{
				"deposit": in.Deposit,
			})
		} else if in.Mint != nil {
			inputs = append(inputs, map[string]any{
				"mint": in.Mint,
			})
		}
	}

	var outputs []map[string]any
	for _, out := range tx.Outputs {
		output := map[string]any{
			"type":   out.Type,
			"amount": out.Amount,
		}
		if len(out.Keys) > 0 {
			output["keys"] = out.Keys
		}
		if len(out.Script) > 0 {
			output["script"] = out.Script
		}
		if out.Mask.HasValue() {
			output["mask"] = out.Mask
		}
		if w := out.Withdrawal; w != nil {
			output["withdrawal"] = map[string]any{
				"address": w.Address,
				"tag":     w.Tag,
			}
		}
		outputs = append(outputs, output)
	}

	tm := map[string]any{
		"version":    tx.Version,
		"asset":      tx.Asset,
		"inputs":     inputs,
		"outputs":    outputs,
		"extra":      hex.EncodeToString(tx.Extra),
		"hash":       tx.PayloadHash(),
		"references": tx.References,
	}
	if as := tx.AggregatedSignature; as != nil {
		tm["aggregated"] = map[string]any{
			"signers":   as.Signers,
			"signature": as.Signature,
		}
	} else if tx.SignaturesMap != nil {
		tm["signatures"] = tx.SignaturesMap
	}
	return tm
}
