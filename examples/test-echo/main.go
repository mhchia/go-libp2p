package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mrand "math/rand"

	golog "github.com/ipfs/go-log"
	crypto "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	gologging "github.com/whyrusleeping/go-logging"

	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"

	host "github.com/libp2p/go-libp2p-host"
)

// type Node struct {
// 	Host host.Host
// 	Dht  dht.IpfsDHT
// }

// pattern: /protocol-name/request-or-response-message/version
const addPeerRequest = "/addPeer/addpeerreq/0.0.1"
const addPeerResponse = "/addPeer/addpeerresp/0.0.1"

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeBasicHost(listenPort int, randseed int64) (host.Host, *dht.IpfsDHT, error) {

	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	var r io.Reader
	// if randseed == 0 {
	// 	r = rand.Reader
	// } else {
	// 	r = mrand.New(mrand.NewSource(randseed))
	// }
	// for convenience
	r = mrand.New(mrand.NewSource(randseed))

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	// log.Printf("priv=%s", priv)
	if err != nil {
		return nil, nil, err
	}

	// Get the peer id
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
	if err != nil {
		return nil, nil, err
	}

	// We've created the identity, now we need to store it to use the bhost constructors
	ps := pstore.NewPeerstore()
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, priv.GetPublic())

	ctx := context.Background()
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{maddr}, pid, ps, nil)
	if err != nil {
		return nil, nil, err
	}

	hostOpts := &bhost.HostOpts{
		NATManager: bhost.NewNATManager(netw),
	}

	basicHost, err := bhost.NewHost(ctx, netw, hostOpts)
	if err != nil {
		return nil, nil, err
	}
	dstore := dsync.MutexWrap(ds.NewMapDatastore())
	// Make the DHT NOTE - Using Client constructor
	dht := dht.NewDHT(ctx, basicHost, dstore)
	// Make the routed host
	routedHost := rhost.Wrap(basicHost, dht)

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := routedHost.Addrs()[0]
	log.Printf("addr=%s\n", addr)
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)

	log.Printf("Now run \"./echo -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)

	return routedHost, dht, nil
}

func printPeers(ps pstore.Peerstore) {
	for index, peerId := range ps.Peers() {
		log.Printf(
			"peer %d: peerId=%s, peerAddrs=%s\n",
			index,
			peerId,
			ps.PeerInfo(peerId).Addrs,
		)
	}
}

func main() {
	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	golog.SetAllLoggers(gologging.INFO) // Change to DEBUG for extra info

	// Parse options from the command line
	nodeNumber := flag.Int("n", 0, "which node")
	listenF := flag.Int("l", 0, "wait for incoming connections")
	target := flag.String("d", "", "target peer to dial")
	seed := flag.Int64("seed", 0, "set random seed for id generation")
	flag.Parse()

	*listenF = 10000 + *nodeNumber
	*seed = int64(*nodeNumber)
	if *nodeNumber == 0 {
		*target = ""
	} else {
		*target = "/ip4/127.0.0.1/tcp/10000/ipfs/QmS5QmciTXXnCUCyxud5eWFenUMAmvAWSDa1c7dvdXRMZ7"
	}
	if *listenF == 0 {
		log.Fatal("Please provide a port to bind on with -l")
	}

	// Make a host that listens on the given multiaddress
	ha, dht, err := makeBasicHost(*listenF, *seed)
	if err != nil {
		log.Fatal(err)
	}

	// Set a stream handler on host A. /echo/1.0.0 is
	// a user-defined protocol name.
	ha.SetStreamHandler(addPeerRequest, func(s net.Stream) {
		remotePeerId := s.Conn().RemotePeer()
		remotePeerMultiaddr := s.Conn().RemoteMultiaddr()
		log.Printf(
			"Got a new stream! from peerId=%s, peerMultiaddr=%s\n",
			remotePeerId,
			remotePeerMultiaddr,
		)
		if err := doEcho(s); err != nil {
			log.Println(err)
			s.Reset()
		} else {
			s.Close()
		}
		ctx := context.Background()
		p, _ := dht.FindPeersConnectedToPeer(ctx, remotePeerId)
		log.Println("p=", <-p)
		log.Println(dht)
		// TODO: should do other checks to become peer
		ha.Peerstore().AddAddr(remotePeerId, remotePeerMultiaddr, pstore.PermanentAddrTTL)
		printPeers(ha.Peerstore())
	})

	if *target == "" {
		log.Println("listening for connections")
		select {} // hang forever
	}
	/**** This is where the listener code ends ****/

	// The following code extracts target's the peer ID from the
	// given multiaddress
	ipfsaddr, err := ma.NewMultiaddr(*target) // ipfsaddr=/ip4/127.0.0.1/tcp/10000/ipfs/QmVmDaabYcS3pn23KaFjkdw6hkReUUma8sBKqSDHrPYPd2
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
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid))) // /ipfs/QmVmDaabYcS3pn23KaFjkdw6hkReUUma8sBKqSDHrPYPd2
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr) // /ip4/127.0.0.1/tcp/10000
	log.Printf("targetAddr=%s\n", targetAddr)

	// We have a peer ID and a targetAddr so we add it to the peerstore
	// so LibP2P knows how to contact it
	ha.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)
	log.Println("my Peerstore=", ha.Peerstore().Peers())
	printPeers(ha.Peerstore())
	log.Println("opening stream")
	// make a new stream from host B to host A
	// it should be handled on host A by the handler we set above because
	// we use the same /echo/1.0.0 protocol
	s, err := ha.NewStream(context.Background(), peerid, addPeerRequest)
	if err != nil {
		log.Fatalln(err)
	}

	_, err = s.Write([]byte("Hello, world!\n"))
	if err != nil {
		log.Fatalln(err)
	}

	out, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("read reply: %q\n", out)
	select {} // hang forever
}

// doEcho reads a line of data a stream and writes it back
func doEcho(s net.Stream) error {
	buf := bufio.NewReader(s)
	str, err := buf.ReadString('\n')
	if err != nil {
		return err
	}

	log.Printf("read: %s\n", str)
	_, err = s.Write([]byte(str))
	return err
}
