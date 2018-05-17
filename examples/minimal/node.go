package main

import (
	"bufio"
	"context"
	"fmt"
	"log"

	"github.com/gogo/protobuf/proto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// node client version
const clientVersion = "go-p2p-node/0.0.1"

// Node type - a p2p host implementing one or more p2p protocols
type Node struct {
	host.Host        // lib-p2p host
	*AddPeerProtocol // addpeer protocol impl

	// Shard related
	// TODO: maybe move all sharding related things to `ShardManager`?
	*ShardManager
	ShardProtocols map[ShardIDType]*ShardProtocol
	// add other protocols here...
}

// Create a new node with its implemented protocols
func NewNode(host host.Host, ctx context.Context) *Node {
	node := &Node{Host: host}
	node.AddPeerProtocol = NewAddPeerProtocol(node)

	node.ShardManager = NewShardManager(node)
	node.ShardProtocols = make(map[ShardIDType]*ShardProtocol, numShards)
	return node
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (n *Node) sendProtoMessage(data proto.Message, s inet.Stream) bool {
	writer := bufio.NewWriter(s)
	enc := protobufCodec.Multicodec(nil).Encoder(writer)
	err := enc.Encode(data)
	if err != nil {
		log.Println(err)
		return false
	}
	writer.Flush()
	// writer.Flush()
	return true
}

func (n *Node) GetFullAddr() string {
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", n.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := n.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	return fullAddr.String()
}

func (n *Node) IsPeer(peerID peer.ID) bool {
	for _, value := range n.Peerstore().Peers() {
		if value == peerID {
			return true
		}
	}
	return false
}

func (n *Node) ListenShard(shardID ShardIDType) {
	if !(n.IsShardListened(shardID)) {
		n.AddPeerListeningShard(n.ID(), shardID)
		n.ShardProtocols[shardID] = NewShardProtocol(n, shardID)
	}
}

func (n *Node) UnlistenShard(shardID ShardIDType) {
	if n.IsShardListened(shardID) {
		n.RemovePeerListeningShard(n.ID(), shardID)
		if _, prs := n.ShardProtocols[shardID]; prs {
			// s.listeningShards[shardID] = false
			delete(n.ShardProtocols, shardID)
		}
		n.RemoveStreamHandler(getSendCollationRequestProtocolID(shardID))
	}
}

func (n *Node) GetListeningShards() []ShardIDType {
	return n.GetPeerListeningShard(n.ID())
}

func InShards(shardID ShardIDType, shards []ShardIDType) bool {
	for _, value := range shards {
		if value == shardID {
			return true
		}
	}
	return false
}

func (n *Node) IsShardListened(shardID ShardIDType) bool {
	return InShards(shardID, n.GetListeningShards())
}
