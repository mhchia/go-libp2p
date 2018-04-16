package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mrand "math/rand"

	libp2p "github.com/libp2p/go-libp2p"
	gologging "gx/ipfs/QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV/go-logging"
	golog "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	net "gx/ipfs/QmXoz9o2PT3tEzf7hicegwex5UgVP54n3k82K7jrWFyN86/go-libp2p-net"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
	pstore "gx/ipfs/QmdeiKhUy1TVGBaKxt7y1QmBDLBdisSrLJ1x58Eoj4PXUh/go-libp2p-peerstore"
	crypto "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
	host "gx/ipfs/QmfZTdmunzKzAGJrSvXXQbQ5kLLUiEMX5vdwux7iXkdk7D/go-libp2p-host"
)

// pattern: /protocol-name/request-or-response-message/version
const addPeerRequest = "/addPeer/addpeerreq/0.0.1"
const addPeerResponse = "/addPeer/addpeerresp/0.0.1"

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeBasicHost(listenPort int, secio bool, randseed int64) (host.Host, error) {

	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}
	// for convenience
	r = mrand.New(mrand.NewSource(randseed))

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	// log.Printf("priv=%s", priv)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
	}

	if !secio {
		opts = append(opts, libp2p.NoEncryption())
	}

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := basicHost.Addrs()[0]
	log.Printf("addr=%s\n", addr)
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	if secio {
		log.Printf("Now run \"./echo -l %d -d %s -secio\" on a different terminal\n", listenPort+1, fullAddr)
	} else {
		log.Printf("Now run \"./echo -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)
	}

	return basicHost, nil
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
	secio := flag.Bool("secio", false, "enable secio")
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
	ha, err := makeBasicHost(*listenF, *secio, *seed)
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
	log.Println("ha.Peerstore=", ha.Peerstore().Peers())
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
