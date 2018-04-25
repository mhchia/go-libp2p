package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	mrand "math/rand"

	gologging "gx/ipfs/QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV/go-logging"
	golog "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/Qma7H6RW8wRrfZpNSXwxYGcd1E149s42FpWNpDNieSVrnU/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	pstore "gx/ipfs/QmeZVQzUrXqaszo24DAoHfGzcmCptN9JyngLkGAiEfk2x7/go-libp2p-peerstore"

	bhost "gx/ipfs/QmPd5qhppUqewTQMfStvNNCFtcxiWGsnE6Vs3va6788gsX/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmPd5qhppUqewTQMfStvNNCFtcxiWGsnE6Vs3va6788gsX/go-libp2p/p2p/host/routed"
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	dsync "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore/sync"
	swarm "gx/ipfs/QmSKrS9EF4V8FpD1d5FUGQiwYLNkXcxKabWgT2aWNVnQie/go-libp2p-swarm"
	dht "gx/ipfs/Qmdm5sm2xHCXNaWdxpjhFeStvSNMRhKQkqpBX7aDcqXtfT/go-libp2p-kad-dht"
)

const numShards = 100

// makeNode creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeNode(listenPort int, randseed int64) (*Node, error) {

	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	r := mrand.New(mrand.NewSource(randseed))

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

	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort))
	if err != nil {
		return nil, err
	}

	// We've created the identity, now we need to store it to use the bhost constructors
	ps := pstore.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, priv.GetPublic())

	// Put all this together
	ctx := context.Background()
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

	// // don't do bootstrap if it is bootstrap node itself
	// if bootstrapPeers[0].ID != pid {
	// 	// connect to the chosen ipfs nodes
	// 	err = bootstrapConnect(ctx, routedHost, bootstrapPeers)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// Bootstrap the host
	err = dht.Bootstrap(ctx)
	if err != nil {
		return nil, err
	}

	// Make a host that listens on the given multiaddress
	node := NewNode(routedHost)

	log.Printf("I am %s\n", node.GetFullAddr())

	return node, nil
}

func printPeers(ps pstore.Peerstore) {
	log.Println("Peer ==================================")
	for index, peerID := range ps.Peers() {
		log.Printf(
			"peer %d: peerId=%s, peerAddrs=%s\n",
			index,
			peerID,
			ps.PeerInfo(peerID).Addrs,
		)
	}
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

func main() {
	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	golog.SetAllLoggers(gologging.INFO) // Change to DEBUG for extra info

	// Parse options from the command line
	target := flag.String("d", "", "target peer to dial")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	flag.Parse()

	listenPort := 10000 + *seed

	node, err := makeNode(int(listenPort), *seed)
	log.Println(node.ID())
	if err != nil {
		log.Fatal(err)
	}

	testShardID := ShardIDType(87)
	node.ListenShard(testShardID)

	if *target == "" {
		log.Println("listening for connections")
		select {} // hang forever
	}

	/**** This is where the listener code ends ****/
	node.AddPeer(*target)
	node.ListenShard(20)
	node.ListenShard(30)
	node.UnlistenShard(20)
	log.Println("listeningShards", node.GetListeningShards())
	node.ShardProtocols[testShardID].sendCollation(
		*target,
		1,
		"blobssssss",
	)
	select {}
}
