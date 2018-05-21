package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"time"

	golog "github.com/ipfs/go-log"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	gologging "github.com/whyrusleeping/go-logging"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

// import "C"

const numShards ShardIDType = 100

// makeNode creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeNode(
	ctx context.Context,
	listenPort int,
	randseed int64,
	bootstrapPeers []pstore.PeerInfo) (*Node, error) {

	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	r := mrand.New(mrand.NewSource(randseed))
	// r := rand.Reader

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	// Get the peer id
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
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
	node := NewNode(ctx, routedHost)

	log.Printf("I am %s\n", node.GetFullAddr())

	return node, nil
}

func parseAddr(addrString string) (peerID peer.ID, protocolAddr ma.Multiaddr) {
	// The following code extracts target's the peer ID from the
	// given multiaddress
	ipfsaddr, err := ma.NewMultiaddr(addrString) // ipfsaddr=/ip4/127.0.0.1/tcp/10000/ipfs/QmVmDaabYcS3pn23KaFjkdw6hkReUUma8sBKqSDHrPYPd2
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS) // pid=QmVmDaabYcS3pn23KaFjkdw6hkReUUma8sBKqSDHrPYPd2
	if err != nil {
		log.Fatalln(err)
	}

	peerid, err := peer.IDB58Decode(pid) // peerid=<peer.ID VmDaab>
	if err != nil {
		log.Fatalln(err)
	}

	// Decapsulate the /ipfs/<peerID> part from the target
	// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
	targetPeerAddr, _ := ma.NewMultiaddr(
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)),
	)
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
	return peerid, targetAddr
}

func RunNode(seed int, target string) {
	listenPort := 10000 + seed

	ctx := context.Background()
	node, err := makeNode(
		ctx,
		int(listenPort),
		int64(seed),
		[]pstore.PeerInfo{},
	)
	log.Println(node.ID())
	if err != nil {
		log.Fatal(err)
	}

	testShardID := ShardIDType(87)
	node.ListenShard(testShardID)

	if target == "" {
		log.Println("listening for connections")
		select {} // hang forever
	}

	/**** This is where the listener code ends ****/
	node.AddPeer(target)
	node.ListenShard(20)
	node.ListenShard(30)
	node.UnlistenShard(20)
	log.Println("listeningShards", node.GetListeningShards())
	node.sendCollation(testShardID, 1, "blobssssss")
	select {}
}

func main() {
	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	golog.SetAllLoggers(gologging.INFO) // Change to DEBUG for extra info

	// Parse options from the command line
	target := flag.String("d", "", "target peer to dial")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	sendCollation := flag.Bool("send", false, "send collations")
	flag.Parse()

	listenPort := 10000 + *seed

	ctx := context.Background()
	node, err := makeNode(ctx, int(listenPort), *seed, []pstore.PeerInfo{})
	log.Println(node.ID())
	if err != nil {
		log.Fatal(err)
	}

	numCollations := 100
	var numListeningShards ShardIDType = 100
	blobSize := int(math.Pow(2, 20) - 100) // roughly 1 MB
	for i := ShardIDType(0); i < numListeningShards; i++ {
		node.ListenShard(i)
	}

	if *target != "" {
		node.AddPeer(*target)
	}

	if !(*sendCollation) {
		log.Println("listening for connections")
		select {} // hang forever
	}

	/**** This is where the listener code ends ****/

	time.Sleep(time.Millisecond * 300)

	// time1 := time.Now()
	for i := ShardIDType(0); i < numListeningShards; i++ {
		go func(shardID ShardIDType) {
			for j := 0; j < numCollations; j++ {
				node.sendCollation(
					shardID,
					int64(j),
					string(make([]byte, blobSize)),
				)
				// TODO: this sleep is needed, to control the speed of sending collations
				//		 and to avoid
				time.Sleep(time.Millisecond * 70)
			}
		}(i)
	}
	select {}
}
