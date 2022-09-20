package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/rpc"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dgraph-io/ristretto"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "mixin"
	app.Usage = "A free, lightning fast and decentralized network for transferring digital assets."
	app.Version = config.BuildVersion
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "node",
			Aliases: []string{"n"},
			Value:   "127.0.0.1:8239",
			Usage:   "the node RPC endpoint",
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
					Value:   7239,
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

	logger.SetLevel(c.Int("log"))
	err := logger.SetFilter(c.String("filter"))
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

	addr := fmt.Sprintf(":%d", c.Int("port"))
	node, err := kernel.SetupNode(custom, store, cache, addr, c.String("dir"))
	if err != nil {
		return err
	}

	go func() {
		server := rpc.NewServer(custom, store, node, c.Int("port")+1000)
		err := server.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()

	if custom.Dev.Profile {
		go http.ListenAndServe(fmt.Sprintf(":%d", c.Int("port")+2000), http.DefaultServeMux)
	}

	return node.Loop()
}

func newCache(conf *config.Custom) (*ristretto.Cache, error) {
	cost := int64(conf.Node.MemoryCacheSize * 1024 * 1024)
	return ristretto.NewCache(&ristretto.Config{
		NumCounters: cost / 1024 * 10,
		MaxCost:     cost,
		BufferItems: 64,
	})
}
