# mixin

The Mixin BFT-DAG network reference implementation, the Trusted Execution Environment is not integrated into this repository yet.

## Get Started

Install golang and setup `GOPATH` following this guide https://golang.org/doc/install.

```bash
$ go get -u github.com/MixinNetwork/mixin
$ cd $GOPATH/src/github.com/MixinNetwork/mixin
$ go build
```

The `mixin` command is both the kernel node and tools to communicate with the node RPC interface.

```
$ ./mixin 

NAME:
   mixin - A free and lightning fast peer-to-peer transactional network for digital assets.

USAGE:
   mixin [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     kernel, k             Start the Mixin Kernel daemon
     setuptestnet          Setup the test nodes and genesis
     createaddress         Create a new Mixin address
     signrawtransaction    Sign a JSON encoded transaction
     sendrawtransaction    Broadcast a hex encoded signed raw transaction
     decoderawtransaction  Decode a raw transaction as JSON
     listsnapshots         List finalized snapshots
     getsnapshot           Get the finalized snapshot by transaction hash
     help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## Mixin Kernel Address

Mixin Kernel address are a pair of ed25519 keys, following the [CryptoNote](https://cryptonote.org/standards/) protocol. To create a new address use the `createaddress` command.

```
$ ./mixin createaddress

address:	XINJkpCdwVk3qFqmS3AAAoTmC5Gm2fR3iRF7Rtt7hayuaLXNrtztS3LGPSxTmq5KQh3KJ2qYXYE5a9w8BWXhZAdsJKXqcvUr
view key:	568302b687a2fa3e8853ff35d99ffdf3817b98170de7b51e43d0dcf4fe30470f
spend key:	7c2b5c97278ed371d75610cccd9681af31b0d99be4adc2d66983f3c455fc9702
```

Share the `address` to receive assets from other Mixin Kernel addresses, and keep `view key` and `spend key` privately and securely.

Both the `view key` and `spend key` are required to spend the assets received from others, and the `view key` iteself is sufficient to decode and view all the transactions sent to `address`.

## Start a Kernel Node

To start a node, create a directory `mixin` for the config and network data files, then put a genesis.json and nodes.json files in it.

```
$ ./mixin help kernel

NAME:
   mixin kernel - Start the Mixin Kernel daemon

USAGE:
   mixin kernel [command options] [arguments...]

OPTIONS:
   --dir value, -d value   the data directory
   --port value, -p value  the peer port to listen (default: 7239)
```

## Public Test Net

We have launched a public test net based on the following genesis input, a Mixin Network is uniquely defined by the geneis input.

```json
[
  {
    "address": "XINW9WzZ14GuFeCKXAZbqfdkTDe8qUE8PVqvaT8Lp1x9mKTcVVnKTyZHfqAJUrK6hpcsRz7Fy4w9o99SGFttLX6oGTEgxDDp",
    "balance": "20000",
    "mask": "7a9a549584065b5496abe900d4c4b8634ec3cf0fb072430b0ad547888e6a090b"
  },
  {
    "address": "XINVnCxzfSPp3paJaqswEZbfnoo4t7oZYFKURyq1JexCwmS2SiN4iENGaoyC3w8UvBw6WQi6hLe358x88zZsNiTD4oSvUt7U",
    "balance": "20000",
    "mask": "544987fabe485b82b5076770ba014dc0efdb4ea9d6f29195f0ed8d3fd887b104"
  },
  {
    "address": "XIND1NybhDPsbW9PxZo5odfr8BgKrFeHhVdU47uQJGVaWr1mB4fwJoCBF8pUN6ByFJi8Y9yYYJHFr7iVwpdB8XZpc1MHRgSH",
    "balance": "20000",
    "mask": "7ae40fc23b410f4a291e86de0feb59d5e1fa0341bb414af797b073ec77670f0c"
  },
  {
    "address": "XINBQdjusJgj8PXU6dLLtkZD4AFyCcFp7yrXPk9hEVaoSM4BGxAQh8PRB31d4oPCMGEDKR49wVcGcJBNhjhHBfgufdmzfjYs",
    "balance": "20000",
    "mask": "84eb77a4dfda809e8a8fbfc35dc103645d5c4ffe29adb06702c424540708e90e"
  },
  {
    "address": "XINPzESkf13gy6YR2wkguodXJUjnFH3qR45wK4uWmJAmV2HnXKF8AsvFurv1m8EJMe7NiGsD87VgrDKJNH6hUEhEnjGKEPHd",
    "balance": "20000",
    "mask": "6763e95e69910a587d34d1d55a97b7dccbfe8886aaede1f92adbd862a2ce3306"
  },
  {
    "address": "XINEB8CSrP3ihc477KsR2CgnmJ2yfNqyviHcYqtzHNyRSqSJw7aYTxFehJFmoLr8RVjqWbwhTTfBVtv7pbFpyYwdFXJnu27C",
    "balance": "20000",
    "mask": "0e8ae5ce4776188793d3949bd9efe19cbc60a457ed4d38c98890f27e317d8f01"
  },
  {
    "address": "XINVyyRAZszyT826EEkDU8AanUNNzaELuBaUm6Spdu5KNCkpoRMiMWUjwrqgWhkQfb6yZ8MY1XUGy1nv3q1gCDpL2pkurgxK",
    "balance": "20000",
    "mask": "d4260863cba4d8d9bd1e23b89215fe0cac309e3325fb0ac77a31f9dcaa243c05"
  }
]
```

The current implementation can't accept new Kernel nodes `pledge` and `reclaim` transaction, will be added in future releases soon. To connect to the public test net, please use the following `nodes.json` config.

```json
[
  {
    "address": "XINEB8CSrP3ihc477KsR2CgnmJ2yfNqyviHcYqtzHNyRSqSJw7aYTxFehJFmoLr8RVjqWbwhTTfBVtv7pbFpyYwdFXJnu27C",
    "host": "35.199.158.116:7239"
  },
  {
    "address": "XINW9WzZ14GuFeCKXAZbqfdkTDe8qUE8PVqvaT8Lp1x9mKTcVVnKTyZHfqAJUrK6hpcsRz7Fy4w9o99SGFttLX6oGTEgxDDp",
    "host": "104.198.89.213:7239"
  },
  {
    "address": "XINVnCxzfSPp3paJaqswEZbfnoo4t7oZYFKURyq1JexCwmS2SiN4iENGaoyC3w8UvBw6WQi6hLe358x88zZsNiTD4oSvUt7U",
    "host": "35.224.44.182:7239"
  },
  {
    "address": "XIND1NybhDPsbW9PxZo5odfr8BgKrFeHhVdU47uQJGVaWr1mB4fwJoCBF8pUN6ByFJi8Y9yYYJHFr7iVwpdB8XZpc1MHRgSH",
    "host": "35.240.170.36:7239"
  },
  {
    "address": "XINBQdjusJgj8PXU6dLLtkZD4AFyCcFp7yrXPk9hEVaoSM4BGxAQh8PRB31d4oPCMGEDKR49wVcGcJBNhjhHBfgufdmzfjYs",
    "host": "35.231.25.31:7239"
  },
  {
    "address": "XINPzESkf13gy6YR2wkguodXJUjnFH3qR45wK4uWmJAmV2HnXKF8AsvFurv1m8EJMe7NiGsD87VgrDKJNH6hUEhEnjGKEPHd",
    "host": "35.221.23.162:7239"
  },
  {
    "address": "XINVyyRAZszyT826EEkDU8AanUNNzaELuBaUm6Spdu5KNCkpoRMiMWUjwrqgWhkQfb6yZ8MY1XUGy1nv3q1gCDpL2pkurgxK",
    "host": "35.235.122.39:7239"
  }
]
```

## Local Test Net

This will setup a minimum local test net, with all nodes in a single device.

```
$ ./mixin setuptestnet

$ ./mixin kernel -dir /tmp/mixin-7001 -port 7001
$ ./mixin kernel -dir /tmp/mixin-7002 -port 7002
$ ./mixin kernel -dir /tmp/mixin-7003 -port 7003
$ ./mixin kernel -dir /tmp/mixin-7004 -port 7004
$ ./mixin kernel -dir /tmp/mixin-7005 -port 7005
$ ./mixin kernel -dir /tmp/mixin-7006 -port 7006
$ ./mixin kernel -dir /tmp/mixin-7007 -port 7007
```
