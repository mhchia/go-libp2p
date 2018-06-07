package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"strconv"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	golog "github.com/ipfs/go-log"
	crypto "github.com/libp2p/go-libp2p-crypto"
	ic "github.com/libp2p/go-libp2p-crypto"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	gologging "github.com/whyrusleeping/go-logging"
)

// import "C"

type ShardIDType = int64

const numShards ShardIDType = 100

func makeKey(seed int64) (ic.PrivKey, peer.ID, error) {
	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	r := mrand.New(mrand.NewSource(seed))
	// r := rand.Reader

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, "", err
	}

	// Get the peer id
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}
	return priv, pid, nil
}

// makeNode creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeNode(
	ctx context.Context,
	listenPort int,
	randseed int64,
	bootstrapPeers []pstore.PeerInfo) (*Node, error) {

	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
	if err != nil {
		return nil, err
	}

	priv, pid, err := makeKey(randseed)
	if err != nil {
		return nil, err
	}

	// We've created the identity, now we need to store it to use the bhost constructors
	ps := pstore.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, priv.GetPublic())

	// Put all this together
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{maddr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	hostOpts := &bhost.HostOpts{
		NATManager: bhost.NewNATManager(netw),
	}

	basicHost, err := bhost.NewHost(ctx, netw, hostOpts)
	if err != nil {
		return nil, err
	}

	// Construct a datastore (needed by the DHT). This is just a simple, in-memory thread-safe datastore.
	dstore := dsync.MutexWrap(ds.NewMapDatastore())

	// Make the DHT
	dht := dht.NewDHT(ctx, basicHost, dstore)

	// Make the routed host
	routedHost := rhost.Wrap(basicHost, dht)

	// try to connect to the chosen nodes
	bootstrapConnect(ctx, routedHost, bootstrapPeers)

	// Bootstrap the host
	err = dht.Bootstrap(ctx)
	if err != nil {
		return nil, err
	}

	// Make a host that listens on the given multiaddress
	node := NewNode(ctx, routedHost, int(randseed))

	log.Printf("I am %s\n", node.GetFullAddr())

	return node, nil
}

const portBase = 10000

func main() {
	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	golog.SetAllLoggers(gologging.INFO) // Change to DEBUG for extra info

	// Parse options from the command line

	targetSeed := flag.Int64("target-seed", -1, "target peer's seed")
	targetIP := flag.String("target-ip", "", "target peer's ip")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	sendCollationOption := flag.String("send", "", "send collations")
	peerSeed := flag.Int64("find", -1, "use dht to find a certain peer with the given peerSeed")
	flag.Parse()

	listenPort := portBase + *seed
	targetPort := portBase + *targetSeed

	ctx := context.Background()
	node, err := makeNode(ctx, int(listenPort), *seed, []pstore.PeerInfo{})

	if err != nil {
		log.Fatal(err)
	}

	if (*targetSeed != -1) && (*targetIP != "") {
		_, targetPID, err := makeKey(*targetSeed)
		mAddr := fmt.Sprintf(
			"/ip4/%s/tcp/%d/ipfs/%s",
			*targetIP,
			targetPort,
			targetPID.Pretty(),
		)
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Millisecond * 500)
		node.AddPeer(mAddr)
		log.Printf(
			"%v dialing to %v",
			node.ID(),
			mAddr,
		)
		time.Sleep(time.Millisecond * 1000)
	}

	if *peerSeed != -1 {
		_, peerID, err := makeKey(*peerSeed)
		if err != nil {
			log.Printf("[ERROR] error parsing peerID in multihash %v", peerID)
		}
		pi := pstore.PeerInfo{
			ID:    peerID,
			Addrs: []ma.Multiaddr{},
		}
		t := time.Now()
		err = node.Connect(context.Background(), pi)
		if err != nil {
			log.Printf("Failed to connect %v", pi.ID)
		} else {
			log.Printf("node.Connect takes %v", time.Since(t))
		}
	}

	var numListeningShards ShardIDType = 100
	for i := ShardIDType(0); i < numListeningShards; i++ {
		node.ListenShard(i)
	}

	time.Sleep(time.Millisecond * 500)

	if *sendCollationOption != "" {
		options := strings.Split(*sendCollationOption, ",")
		if len(options) != 3 {
			log.Fatalf("wrong collation option %v", *sendCollationOption)
		}
		numSendShards, err := strconv.Atoi(options[0])
		if err != nil {
			log.Fatalf("wrong send shards %v", options)
		}
		numCollations, err := strconv.Atoi(options[1])
		if err != nil {
			log.Fatalf("wrong send shards %v", options)
		}
		timeInMs, err := strconv.Atoi(options[2])
		if err != nil {
			log.Fatalf("wrong send shards %v", options)
		}

		blobSize := int(math.Pow(2, 20) - 100) // roughly 1 MB

		/**** This is where the listener code ends ****/

		for i := 0; i < numSendShards; i++ {
			go func(shardID int) {
				for j := 0; j < numCollations; j++ {
					node.SendCollation(
						ShardIDType(shardID),
						int64(j),
						string(make([]byte, blobSize)),
					)
					// TODO: control the speed of sending collations
					time.Sleep(time.Millisecond * time.Duration(timeInMs))
				}
			}(i)
		}
	}
	log.Printf("%v: listening for connections", node.Name())
	select {}
	// for {
	// 	log.Println(node.Name())
	// 	time.Sleep(time.Millisecond * 1000)
	// }
}
