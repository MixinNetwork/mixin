package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/rpc"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/urfave/cli/v2"
)

func main() {
	defaultRPC := os.Getenv("MIXIN_KERNEL_RPC")
	if defaultRPC == "" {
		defaultRPC = "http://127.0.0.1:6860"
	}
	if strings.Contains(config.BuildVersion, "BUILD_VERSION") {
		panic("please build the application using make command.")
	}

	app := cli.NewApp()
	app.Name = "mixin"
	app.Usage = "A free, lightning fast and decentralized network for transferring digital assets."
	app.Version = config.BuildVersion
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "node",
			Aliases: []string{"n"},
			Value:   defaultRPC,
			Usage:   "the RPC endpoint, and the default value is read from environment variable MIXIN_KERNEL_RPC",
		},
		&cli.StringFlag{
			Name:    "dir",
			Aliases: []string{"d"},
			Usage:   "the data directory",
		},
		&cli.BoolFlag{
			Name:  "time",
			Value: false,
			Usage: "print the runtime",
		},
	}
	app.EnableBashCompletion = true
	app.Commands = []*cli.Command{
		{
			Name:    "kernel",
			Aliases: []string{"k"},
			Usage:   "Start the Mixin Kernel daemon",
			Action:  kernelCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "dir",
					Aliases: []string{"d"},
					Usage:   "the data directory",
				},
				&cli.IntFlag{
					Name:    "port",
					Aliases: []string{"p"},
					Value:   123,
					Usage:   "the peer port to listen",
				},
				&cli.IntFlag{
					Name:    "log",
					Aliases: []string{"l"},
					Value:   logger.INFO,
					Usage:   "the log level",
				},
				&cli.StringFlag{
					Name:  "filter",
					Usage: "the RE2 regex pattern to filter log",
				},
			},
		},
		{
			Name:   "setuptestnet",
			Usage:  "Setup the test nodes and genesis",
			Action: setupTestNetCmd,
		},
		{
			Name:   "createaddress",
			Usage:  "Create a new Mixin address",
			Action: createAddressCmd,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "public",
					Usage: "whether mark all my transactions public",
				},
				&cli.StringFlag{
					Name:  "view",
					Usage: "the private view key `HEX` instead of a random one",
				},
				&cli.StringFlag{
					Name:  "spend",
					Usage: "the private spend key `HEX` instead of a random one",
				},
				&cli.StringFlag{
					Name:  "prefix",
					Usage: "a string prefix the final address should have",
				},
				&cli.StringFlag{
					Name:  "suffix",
					Usage: "a string suffix the final address should have",
				},
			},
		},
		{
			Name:   "decodeaddress",
			Usage:  "Decode an address as public view key and public spend key",
			Action: decodeAddressCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "address",
					Aliases: []string{"a"},
					Usage:   "the Mixin Kernel address",
				},
			},
		},
		{
			Name:   "decodesignature",
			Usage:  "Decode a signature",
			Action: decodeSignatureCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "signature",
					Usage: "the cosi signature `HEX`",
				},
			},
		},
		{
			Name:   "decryptghostkey",
			Usage:  "Decrypt a ghost key with the private view key",
			Action: decryptGhostCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "view",
					Usage: "the private view key",
				},
				&cli.StringFlag{
					Name:  "key",
					Usage: "the ghost key",
				},
				&cli.StringFlag{
					Name:  "mask",
					Usage: "the ghost mask",
				},
				&cli.Uint64Flag{
					Name:    "index",
					Aliases: []string{"i"},
					Usage:   "the output index",
				},
			},
		},
		{
			Name:   "updateheadreference",
			Usage:  "Update the cache round external reference, never use it unless agree by other nodes",
			Action: updateHeadReference,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "id",
					Usage: "self node `ID`",
				},
				&cli.Uint64Flag{
					Name:  "round",
					Usage: "self cache round `NUMBER`",
				},
				&cli.StringFlag{
					Name:  "external",
					Usage: "the external reference `HEX`",
				},
			},
		},
		{
			Name:   "removegraphentries",
			Usage:  "Remove data entries by prefix from the graph data storage",
			Action: removeGraphEntries,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "prefix",
					Usage: "the entry prefix",
				},
			},
		},
		{
			Name:   "validategraphentries",
			Usage:  "Validate transaction hash integration",
			Action: validateGraphEntries,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:  "depth",
					Value: 1000,
					Usage: "the maximum round depth to validate for each node",
				},
			},
		},
		{
			Name:   "buildrawtransaction",
			Usage:  "Build a script raw transaction",
			Action: buildRawTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "asset",
					Usage: "the asset id of the transaction",
				},
				&cli.StringFlag{
					Name:  "extra",
					Usage: "the extra of the transaction",
				},
				&cli.StringFlag{
					Name:  "inputs",
					Usage: "the inputs of the transaction",
				},
				&cli.StringFlag{
					Name:  "outputs",
					Usage: "the outputs of the transaction",
				},
				&cli.StringFlag{
					Name:  "view",
					Usage: "the private view key to sign the transaction",
				},
				&cli.StringFlag{
					Name:  "spend",
					Usage: "the private spend key to sign the transaction",
				},
				&cli.StringFlag{
					Name:  "seed",
					Usage: "the mask seed to hide the recipient public key",
				},
			},
		},
		{
			Name:   "signrawtransaction",
			Usage:  "Sign a JSON encoded transaction",
			Action: signTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "raw",
					Usage: "the JSON encoded raw transaction",
				},
				&cli.StringSliceFlag{
					Name:  "key",
					Usage: "the private key to sign the raw transaction",
				},
				&cli.StringFlag{
					Name:  "seed",
					Usage: "the mask seed to hide the recipient public key",
				},
			},
		},
		{
			Name:   "sendrawtransaction",
			Usage:  "Broadcast a hex encoded signed raw transaction",
			Action: sendTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "raw",
					Usage: "the hex encoded signed raw transaction",
				},
			},
		},
		{
			Name:   "decoderawtransaction",
			Usage:  "Decode a raw transaction as JSON",
			Action: decodeTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "raw",
					Usage: "the JSON encoded raw transaction",
				},
			},
		},
		{
			Name:   "signcustodiandeposit",
			Usage:  "Sign a deposit transaction with a single custodian key",
			Action: custodianDepositCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "receiver",
					Usage: "the receiver address of the deposit",
				},
				&cli.StringFlag{
					Name:  "custodian",
					Usage: "the custodian private view and spend key hex",
				},
				&cli.StringFlag{
					Name:  "asset",
					Usage: "the deposit asset id",
				},
				&cli.StringFlag{
					Name:  "chain",
					Usage: "the deposit chain id",
				},
				&cli.StringFlag{
					Name:  "asset_key",
					Usage: "the deposit asset key",
				},
				&cli.StringFlag{
					Name:  "transaction",
					Usage: "the deposit transaction hash",
				},
				&cli.Uint64Flag{
					Name:  "index",
					Usage: "the deposit transaction output index",
				},
				&cli.StringFlag{
					Name:  "amount",
					Usage: "the deposit amount",
				},
			},
		},
		{
			Name:   "buildnodepledgetransaction",
			Usage:  "Build the transaction to pledge a node",
			Action: pledgeNodeCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "view",
					Usage: "the private view key to sign the transaction",
				},
				&cli.StringFlag{
					Name:  "spend",
					Usage: "the private spend key to sign the transaction",
				},
				&cli.StringFlag{
					Name:  "signer",
					Usage: "the signer address",
				},
				&cli.StringFlag{
					Name:  "payee",
					Usage: "the payee address",
				},
				&cli.StringFlag{
					Name:  "input",
					Usage: "the input transaction hash",
				},
				&cli.StringFlag{
					Name:  "amount",
					Value: common.KernelNodePledgeAmount.String(),
					Usage: "the input amount",
				},
			},
		},
		{
			Name:   "buildnodecanceltransaction",
			Usage:  "Build the transaction to cancel a pledging node",
			Action: cancelNodeCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "view",
					Usage: "the private view key which signs the pledging transaction",
				},
				&cli.StringFlag{
					Name:  "spend",
					Usage: "the private spend key which signs the pledging transaction",
				},
				&cli.StringFlag{
					Name:  "receiver",
					Usage: "the address to receive the refund",
				},
				&cli.StringFlag{
					Name:  "pledge",
					Usage: "the hex of raw pledge transaction",
				},
				&cli.StringFlag{
					Name:  "source",
					Usage: "the hex of raw pledging input transaction",
				},
			},
		},
		{
			Name:   "decodenodepledgetransaction",
			Usage:  "Decode the extra info of a pledge transaction",
			Action: decodePledgeNodeCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "raw",
					Usage: "the raw pledge transaction",
				},
			},
		},
		{
			Name:   "encodecustodianextra",
			Usage:  "Encode the custodian node transaction extra",
			Action: encodeCustodianExtraCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "signer",
					Usage: "the private spend key of the kernel signer",
				},
				&cli.StringFlag{
					Name:  "payee",
					Usage: "the private spend key of the kernel payee",
				},
				&cli.StringFlag{
					Name:  "custodian",
					Usage: "the private spend key of the custodian node",
				},
				&cli.StringFlag{
					Name:  "network",
					Usage: "the network id",
				},
			},
		},
		{
			Name:   "getroundlink",
			Usage:  "Get the latest link between two nodes",
			Action: getRoundLinkCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "from",
					Usage: "the reference head",
				},
				&cli.StringFlag{
					Name:  "to",
					Usage: "the reference tail",
				},
			},
		},
		{
			Name:   "getroundbynumber",
			Usage:  "Get a specific round",
			Action: getRoundByNumberCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "id",
					Usage: "the round node id",
				},
				&cli.Uint64Flag{
					Name:  "number",
					Value: 0,
					Usage: "the round number",
				},
			},
		},
		{
			Name:   "getroundbyhash",
			Usage:  "Get a specific round",
			Action: getRoundByHashCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the round hash",
				},
			},
		},
		{
			Name:   "listsnapshots",
			Usage:  "List finalized snapshots",
			Action: listSnapshotsCmd,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:    "since",
					Aliases: []string{"s"},
					Value:   0,
					Usage:   "the topological order to begin with",
				},
				&cli.Uint64Flag{
					Name:    "count",
					Aliases: []string{"c"},
					Value:   10,
					Usage:   "the up limit of the returned snapshots",
				},
				&cli.BoolFlag{
					Name:  "sig",
					Usage: "whether including the signatures",
				},
				&cli.BoolFlag{
					Name:  "tx",
					Usage: "whether including the transactions",
				},
			},
		},
		{
			Name:   "getsnapshot",
			Usage:  "Get the snapshot by hash",
			Action: getSnapshotCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the snapshot hash",
				},
			},
		},
		{
			Name:   "gettransaction",
			Usage:  "Get the finalized transaction by hash",
			Action: getTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the transaction hash",
				},
			},
		},
		{
			Name:   "getcachetransaction",
			Usage:  "Get the transaction in cache by hash",
			Action: getCacheTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the transaction hash",
				},
			},
		},
		{
			Name:   "getdeposittransaction",
			Usage:  "Get the deposit transaction by external chain transaction",
			Action: getDepositTransactionCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "chain",
					Usage: "the chain hash",
				},
				&cli.StringFlag{
					Name:  "hash",
					Usage: "the external chain transaction hash",
				},
				&cli.IntFlag{
					Name:  "index",
					Usage: "the external chain transaction output index",
				},
			},
		},
		{
			Name:   "getwithdrawalclaim",
			Usage:  "Get the claim transaction for a withdrawal submit",
			Action: getWithdrawalClaimCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the withdrawal submit transaction hash",
				},
			},
		},
		{
			Name:   "getutxo",
			Usage:  "Get the UTXO by hash and index",
			Action: getUTXOCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "hash",
					Aliases: []string{"x"},
					Usage:   "the transaction hash",
				},
				&cli.Uint64Flag{
					Name:    "index",
					Aliases: []string{"i"},
					Value:   0,
					Usage:   "the output index",
				},
			},
		},
		{
			Name:   "getkey",
			Usage:  "Get the ghost key",
			Action: getKeyCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "key",
					Aliases: []string{"k"},
					Usage:   "the ghost key",
				},
			},
		},
		{
			Name:   "getasset",
			Usage:  "Get the asset and balance",
			Action: getAssetCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "id",
					Usage: "the asset id",
				},
			},
		},
		{
			Name:   "listcustodianupdates",
			Usage:  "List all custodian updates",
			Action: listCustodianUpdatesCmd,
			Flags:  []cli.Flag{},
		},
		{
			Name:   "listmintworks",
			Usage:  "List mint works",
			Action: listMintWorksCmd,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:    "since",
					Aliases: []string{"s"},
					Value:   0,
					Usage:   "the mint batch to list works",
				},
			},
		},
		{
			Name:   "listmintdistributions",
			Usage:  "List mint distributions",
			Action: listMintDistributionsCmd,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:    "since",
					Aliases: []string{"s"},
					Value:   0,
					Usage:   "the mint batch to begin with",
				},
				&cli.Uint64Flag{
					Name:    "count",
					Aliases: []string{"c"},
					Value:   10,
					Usage:   "the up limit of the returned distributions",
				},
				&cli.BoolFlag{
					Name:  "tx",
					Usage: "whether including the transactions",
				},
			},
		},
		{
			Name:   "listallnodes",
			Usage:  "List all nodes ever existed",
			Action: listAllNodesCmd,
			Flags: []cli.Flag{
				&cli.Uint64Flag{
					Name:  "threshold",
					Value: 0,
					Usage: "the threshold in Unix nanoseconds to build the nodes list",
				},
				&cli.BoolFlag{
					Name:  "state",
					Value: false,
					Usage: "whether keep a full state queue",
				},
			},
		},
		{
			Name:   "getinfo",
			Usage:  "Get info from the node",
			Action: getInfoCmd,
		},
		{
			Name:   "listpeers",
			Usage:  "List all the connected peers",
			Action: listPeersCmd,
		},
		{
			Name:   "listrelayers",
			Usage:  "List the remote relayers for peer",
			Action: listRelayersCmd,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "id",
					Usage: "the peer node id",
				},
			},
		},
		{
			Name:   "dumpgraphhead",
			Usage:  "Dump the graph head",
			Action: dumpGraphHeadCmd,
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

func kernelCmd(c *cli.Context) error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	err := os.Setenv("QUIC_GO_DISABLE_GSO", "true")
	if err != nil {
		return err
	}

	logger.SetLevel(c.Int("log"))
	err = logger.SetFilter(c.String("filter"))
	if err != nil {
		return err
	}

	gns, err := common.ReadGenesis(c.String("dir") + "/genesis.json")
	if err != nil {
		return err
	}

	custom, err := config.Initialize(c.String("dir") + "/config.toml")
	if err != nil {
		return err
	}

	cache, err := newCache(custom)
	if err != nil {
		return err
	}

	store, err := storage.NewBadgerStore(custom, c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()

	node, err := kernel.SetupNode(custom, store, cache, gns)
	if err != nil {
		return err
	}

	if p := custom.RPC.Port; p > 0 {
		server := rpc.NewServer(custom, store, node, p)
		go func() {
			err := server.ListenAndServe()
			if err != nil {
				panic(err)
			}
		}()
	}

	if p := custom.Dev.Port; p > 0 {
		go func() {
			err := http.ListenAndServe(fmt.Sprintf(":%d", p), http.DefaultServeMux)
			if err != nil {
				panic(err)
			}
		}()
	}

	return node.Loop()
}

func newCache(conf *config.Custom) (*ristretto.Cache[[]byte, any], error) {
	cost := int64(conf.Node.MemoryCacheSize * 1024 * 1024)
	return ristretto.NewCache(&ristretto.Config[[]byte, any]{
		NumCounters: cost / 1024 * 10,
		MaxCost:     cost,
		BufferItems: 64,
	})
}
