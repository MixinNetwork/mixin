package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func commandContext(t *testing.T, args ...string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	for _, name := range []string{"view", "spend", "prefix", "suffix", "seed", "asset", "extra", "inputs", "outputs", "raw", "node"} {
		set.String(name, "", "")
	}
	set.Bool("public", false, "")
	set.Var(cli.NewStringSlice(), "key", "")
	require.NoError(t, set.Parse(args))
	return cli.NewContext(cli.NewApp(), set, nil)
}

func commandOutput(t *testing.T, call func() error) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "output")
	require.NoError(t, err)
	previous := os.Stdout
	os.Stdout = f
	defer func() {
		os.Stdout = previous
		f.Close()
	}()
	require.NoError(t, call())
	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	return string(data)
}

func TestCreateAddressRejectsMalformedPrivateKeys(t *testing.T) {
	for _, name := range []string{"view", "spend"} {
		for _, key := range []string{"01", strings.Repeat("01", 33), strings.Repeat("ff", 32)} {
			t.Run(name+"/"+key, func(t *testing.T) {
				require.Error(t, createAddressCmd(commandContext(t, "--"+name, key)))
			})
		}
	}
}

func TestCreateAddressUsesExactPrivateKeys(t *testing.T) {
	account := common.NewAddressFromSeed(bytes.Repeat([]byte{31}, 64))
	args := []string{"--view", account.PrivateViewKey.String(), "--spend", account.PrivateSpendKey.String()}
	output := commandOutput(t, func() error { return createAddressCmd(commandContext(t, args...)) })
	require.Contains(t, output, account.String())
	require.Contains(t, output, account.PrivateViewKey.String())
	require.Contains(t, output, account.PrivateSpendKey.String())
	args = append(args, "--prefix", "impossible")
	require.ErrorContains(t, createAddressCmd(commandContext(t, args...)), "does not match")
}

func TestSignTransactionPreservesReferencesAndWithdrawal(t *testing.T) {
	account := common.NewAddressFromSeed(bytes.Repeat([]byte{32}, 64))
	funding := common.NewTransactionV5(common.XINAssetId)
	funding.AddInput(crypto.Blake3Hash([]byte("funding")), 0)
	funding.AddScriptOutput([]*common.Address{&account}, common.NewThresholdScript(1), common.NewInteger(1), bytes.Repeat([]byte{33}, 64))
	utxo := funding.AsVersioned().UnspentOutputs()[0]
	reference := crypto.Blake3Hash([]byte("reference"))
	withdrawal := &common.WithdrawalData{Address: "destination", Tag: "memo"}
	raw, err := json.Marshal(map[string]any{
		"version":    common.TxVersionHashSignature,
		"asset":      common.XINAssetId,
		"inputs":     []any{map[string]any{"hash": utxo.Hash, "index": utxo.Index, "keys": utxo.Keys, "mask": utxo.Mask}},
		"outputs":    []any{map[string]any{"type": common.OutputTypeWithdrawalSubmit, "amount": common.NewInteger(1), "withdrawal": withdrawal}},
		"references": []crypto.Hash{reference},
	})
	require.NoError(t, err)
	output := commandOutput(t, func() error {
		return signTransactionCmd(commandContext(t, "--raw", string(raw), "--key", account.PrivateViewKey.String()+account.PrivateSpendKey.String()))
	})
	payload, err := hex.DecodeString(strings.TrimSpace(output))
	require.NoError(t, err)
	ver, err := common.UnmarshalVersionedTransaction(payload)
	require.NoError(t, err)
	require.Equal(t, []crypto.Hash{reference}, ver.References)
	require.Equal(t, withdrawal, ver.Outputs[0].Withdrawal)
	require.True(t, utxo.Keys[0].Verify(ver.PayloadHash(), *ver.SignaturesMap[0][0]))
}

func TestBuildRawTransactionRejectsInvalidIndexAndAmount(t *testing.T) {
	account := common.NewAddressFromSeed(bytes.Repeat([]byte{34}, 64))
	hash := crypto.Blake3Hash([]byte("input"))
	for _, test := range []struct{ index, amount string }{{"-1", "1"}, {"1025", "1"}, {"65535", "1"}, {"65536", "1"}, {"0", "invalid"}, {"0", "-1"}} {
		t.Run(test.index+"/"+test.amount, func(t *testing.T) {
			ctx := commandContext(t, "--view", account.PrivateViewKey.String(), "--spend", account.PrivateSpendKey.String(),
				"--asset", common.XINAssetId.String(), "--inputs", hash.String()+":"+test.index, "--outputs", account.String()+":"+test.amount)
			require.Error(t, buildRawTransactionCmd(ctx))
		})
	}
}

func TestSignTransactionRejectsInvalidConstruction(t *testing.T) {
	key := crypto.NewKeyFromSeed(bytes.Repeat([]byte{35}, 64)).Public()
	for _, test := range []struct{ name, fields string }{
		{"index", `"inputs":[{"index":1025}]`},
		{"null account", `"outputs":[{"accounts":[null]}]`},
		{"null key", `"outputs":[{"keys":[null]}]`},
		{"input count", `"inputs":[` + strings.TrimSuffix(strings.Repeat(`{},`, common.SliceCountLimit+1), ",") + `]`},
		{"output count", `"outputs":[` + strings.TrimSuffix(strings.Repeat(`{},`, common.SliceCountLimit+1), ",") + `]`},
		{"references", `"references":[` + strings.TrimSuffix(strings.Repeat(`"`+(crypto.Hash{}).String()+`",`, common.ReferencesCountLimit+1), ",") + `]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := `{"version":5,` + test.fields + `}`
			require.Error(t, signTransactionCmd(commandContext(t, "--raw", raw)))
		})
	}
	for _, data := range []string{
		`{"inputs":[{"keys":[null],"mask":"` + key.String() + `"}]}`,
		`{"inputs":[{"keys":["` + key.String() + `"]}]}`,
	} {
		var raw signerInput
		require.NoError(t, json.Unmarshal([]byte(data), &raw))
		_, err := raw.ReadUTXOKeys(crypto.Hash{}, 0)
		require.Error(t, err)
	}
}
