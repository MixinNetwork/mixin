package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/rpc"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/urfave/cli"
)

func init() {
	logger.Init(config.LoggerLevel)
}

func main() {
	app := cli.NewApp()
	app.Name = "mixin"
	app.Usage = "A free and lightning fast peer-to-peer transactional network for digital assets."
	app.Version = config.BuildVersion
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		{
			Name:    "kernel",
			Aliases: []string{"k"},
			Usage:   "Start the Mixin Kernel daemon",
			Action:  kernelCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "dir,d",
					Usage: "the data directory",
				},
				cli.IntFlag{
					Name:  "port,p",
					Value: 7239,
					Usage: "the peer port to listen",
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
			Action: createAdressCmd,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "public",
					Usage: "whether mark all my transactions public",
				},
				cli.StringFlag{
					Name:  "view",
					Usage: "the private view key `HEX` instead of a random one",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "address,a",
					Usage: "the Mixin Kernel address",
				},
			},
		},
		{
			Name:   "decryptghostkey",
			Usage:  "Decrypt a ghost key with the private view key",
			Action: decryptGhostCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "view",
					Usage: "the private view key",
				},
				cli.StringFlag{
					Name:  "key",
					Usage: "the ghost key",
				},
				cli.StringFlag{
					Name:  "mask",
					Usage: "the ghost mask",
				},
				cli.Uint64Flag{
					Name:  "index",
					Usage: "the output index",
				},
			},
		},
		{
			Name:   "updateheadreference",
			Usage:  "Update the cache round external reference, never use it unless agree by other nodes",
			Action: updateHeadReference,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "dir",
					Usage: "the data directory",
				},
				cli.StringFlag{
					Name:  "node",
					Usage: "self node `ID`",
				},
				cli.Uint64Flag{
					Name:  "round",
					Usage: "self cache round `NUMBER`",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "dir",
					Usage: "the data directory",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "dir",
					Usage: "the data directory",
				},
			},
		},
		{
			Name:   "signrawtransaction",
			Usage:  "Sign a JSON encoded transaction",
			Action: signTransactionCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "raw",
					Usage: "the JSON encoded raw transaction",
				},
				cli.StringSliceFlag{
					Name:  "key",
					Usage: "the private key to sign the raw transaction",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "raw",
					Usage: "the JSON encoded raw transaction",
				},
			},
		},
		{
			Name:   "buildnodecanceltransaction",
			Usage:  "Build the transaction to cancel a pledging node",
			Action: cancelNodeCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "view",
					Usage: "the private view key which signs the pledging transaction",
				},
				cli.StringFlag{
					Name:  "spend",
					Usage: "the private spend key which signs the pledging transaction",
				},
				cli.StringFlag{
					Name:  "receiver",
					Usage: "the address to receive the refund",
				},
				cli.StringFlag{
					Name:  "pledge",
					Usage: "the hex of raw pledge transaction",
				},
				cli.StringFlag{
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
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "from",
					Usage: "the reference head",
				},
				cli.StringFlag{
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
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "id",
					Usage: "the round node id",
				},
				cli.Uint64Flag{
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
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "hash",
					Usage: "the round hash",
				},
			},
		},
		{
			Name:   "listsnapshots",
			Usage:  "List finalized snapshots",
			Action: listSnapshotsCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.Uint64Flag{
					Name:  "since,s",
					Value: 0,
					Usage: "the topological order to begin with",
				},
				cli.Uint64Flag{
					Name:  "count,c",
					Value: 10,
					Usage: "the up limit of the returned snapshots",
				},
				cli.BoolFlag{
					Name:  "sig",
					Usage: "whether including the signatures",
				},
				cli.BoolFlag{
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
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "hash,x",
					Usage: "the snapshot hash",
				},
			},
		},
		{
			Name:   "gettransaction",
			Usage:  "Get the finalized transaction by hash",
			Action: getTransactionCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "hash,x",
					Usage: "the transaction hash",
				},
			},
		},
		{
			Name:   "getutxo",
			Usage:  "Get the UTXO by hash and index",
			Action: getUTXOCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.StringFlag{
					Name:  "hash,x",
					Usage: "the transaction hash",
				},
				cli.Uint64Flag{
					Name:  "index,i",
					Value: 0,
					Usage: "the output index",
				},
			},
		},
		{
			Name:   "listmintdistributions",
			Usage:  "List mint distributions",
			Action: listMintDistributionsCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
				cli.Uint64Flag{
					Name:  "since,s",
					Value: 0,
					Usage: "the mint batch to begin with",
				},
				cli.Uint64Flag{
					Name:  "count,c",
					Value: 10,
					Usage: "the up limit of the returned distributions",
				},
				cli.BoolFlag{
					Name:  "tx",
					Usage: "whether including the transactions",
				},
			},
		},
		{
			Name:   "getinfo",
			Usage:  "Get info from the node",
			Action: getInfoCmd,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "node,n",
					Value: "127.0.0.1:8239",
					Usage: "the node RPC endpoint",
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

func kernelCmd(c *cli.Context) error {
	runtime.GOMAXPROCS(runtime.NumCPU())

	err := config.Initialize(c.String("dir") + "/config.json")
	if err != nil {
		return err
	}

	cache := fastcache.New(config.Custom.MaxCacheSize * 1024 * 1024)
	go func() {
		var s fastcache.Stats
		for {
			time.Sleep(1 * time.Minute)
			cache.UpdateStats(&s)
			logger.Printf("CACHE STATS GET: %d SET: %d COLLISION: %d SIZE: %dMB\n", s.GetCalls, s.SetCalls, s.Collisions, s.BytesSize/1024/1024)
			s.Reset()
		}
	}()

	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()

	addr := fmt.Sprintf(":%d", c.Int("port"))
	node, err := kernel.SetupNode(store, cache, addr, c.String("dir"))
	if err != nil {
		return err
	}

	go func() {
		err := node.Loop()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := rpc.StartHTTP(store, node, c.Int("port")+1000)
		if err != nil {
			panic(err)
		}
	}()
	return http.ListenAndServe(fmt.Sprintf(":%d", c.Int("port")+2000), http.DefaultServeMux)
}
