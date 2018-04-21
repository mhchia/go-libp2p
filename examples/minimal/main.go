package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	mrand "math/rand"

	net "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	gologging "gx/ipfs/QmQvJiADDe7JR4m968MwXobTCCzUqQkP87aRHe29MEBGHV/go-logging"
	golog "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/Qma7H6RW8wRrfZpNSXwxYGcd1E149s42FpWNpDNieSVrnU/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	pstore "gx/ipfs/QmeZVQzUrXqaszo24DAoHfGzcmCptN9JyngLkGAiEfk2x7/go-libp2p-peerstore"

	host "gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	libp2p "github.com/libp2p/go-libp2p"
	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It will use secio if secio is true.
func makeBasicHost(listenPort int, randseed int64) (host.Host, error) {

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

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		libp2p.Identity(priv),
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
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	log.Printf("Now run \"./echo -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)

	return basicHost, nil
}

const addPeerRequest = "/addPeer/addpeerreq"
const addPeerResponse = "/addPeer/addpeerresp"

func printPeers(ps pstore.Peerstore) {
	for index, peerID := range ps.Peers() {
		log.Printf(
			"peer %d: peerId=%s, peerAddrs=%s\n",
			index,
			peerID,
			ps.PeerInfo(peerID).Addrs,
		)
	}
}

func handleAddPeer(h host.Host, s net.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	remotePeerMultiaddr := s.Conn().RemoteMultiaddr()
	log.Printf(
		"%s: from peerId=%s, peerMultiaddr=%s\n",
		addPeerRequest,
		remotePeerID,
		remotePeerMultiaddr,
	)

	log.Println("tring to parse message")
	data := &pbmsg.AddPeerRequest{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		return
	}

	if err := doEcho(s); err != nil {
		log.Println(err)
		s.Reset()
	} else {
		s.Close()
	}
	// TODO: should do other checks to become peer
	// h.Peerstore().AddAddr(remotePeerID, remotePeerMultiaddr, pstore.PermanentAddrTTL)
	s.Reset()
	printPeers(h.Peerstore())
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
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)
	return peerid, targetAddr
}

func sendAddPeer(h host.Host, peerAddr string) {

	peerid, targetAddr := parseAddr(peerAddr)
	// TODO: seems `Peerstore().AddAddr` must be done before we open a new stream?
	// We have a peer ID and a targetAddr so we add it to the peerstore
	// so LibP2P knows how to contact it
	h.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)

	log.Println("opening stream")
	// make a new stream from host B to host A
	// it should be handled on host A by the handler we set above because
	// we use the same /echo/1.0.0 protocol
	s, err := h.NewStream(context.Background(), peerid, addPeerRequest)

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

	// Make a host that listens on the given multiaddress
	ha, err := makeBasicHost(int(listenPort), *seed)
	if err != nil {
		log.Fatal(err)
	}

	// Set a stream handler on host A. /echo/1.0.0 is
	// a user-defined protocol name.
	ha.SetStreamHandler(addPeerRequest, func(s net.Stream) {
		handleAddPeer(ha, s)
	})

	if *target == "" {
		log.Println("listening for connections")
		select {} // hang forever
	}

	/**** This is where the listener code ends ****/
	sendAddPeer(ha, *target)
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
