package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/rpc"
	"github.com/MixinNetwork/mixin/storage"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "mixin"
	app.Usage = "A free and lightning fast peer-to-peer transactional network for digital assets."
	app.Version = "0.0.1"
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
				cli.StringFlag{
					Name:  "key",
					Usage: "the private key to sign the raw transaction",
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
					Value: 100,
					Usage: "the up limit of the returned snapshots",
				},
				cli.BoolFlag{
					Name:  "sig",
					Usage: "whether including the signatures",
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
		panic(err)
	}
}

func kernelCmd(c *cli.Context) error {
	runtime.GOMAXPROCS(128)

	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()

	go func() {
		err := rpc.StartHTTP(store, c.Int("port")+1000)
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", c.Int("port")+2000), http.DefaultServeMux)
		if err != nil {
			panic(err)
		}
	}()

	return kernel.Loop(store, fmt.Sprintf(":%d", c.Int("port")), c.String("dir"))
}
