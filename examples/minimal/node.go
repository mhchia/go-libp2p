package main

import (
	"bufio"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	host "gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// node client version
const clientVersion = "go-p2p-node/0.0.1"

type ShardIDType int64

type ShardManager struct {
	listeningShards map[ShardIDType]bool
	PeerShards      map[string]([]ShardIDType)
}

// Node type - a p2p host implementing one or more p2p protocols
type Node struct {
	host.Host        // lib-p2p host
	*AddPeerProtocol // addpeer protocol impl

	// Shard related
	ShardManager          ShardManager
	*NotifyShardsProtocol // notifyshards protocol
	ShardProtocols        map[ShardIDType]*ShardProtocol
	// add other protocols here...
}

// Create a new node with its implemented protocols
func NewNode(host host.Host) *Node {
	node := &Node{Host: host}
	node.AddPeerProtocol = NewAddPeerProtocol(node)
	node.initShardManager()
	node.NotifyShardsProtocol = NewNotifyShardsProtocol(node)
	node.ShardProtocols = make(map[ShardIDType]*ShardProtocol, numShards)
	return node
}

func (n *Node) initShardManager() {
	n.ShardManager.listeningShards = make(map[ShardIDType]bool, numShards)
}

func (n *Node) AddListeningShard(shardID ShardIDType) {
	n.ShardManager.listeningShards[shardID] = true
	n.ShardProtocols[shardID] = NewShardProtocol(n, shardID)
}

func (n *Node) GetListeningShards() []ShardIDType {
	listeningShards := []ShardIDType{}
	for shardID, isListening := range n.ShardManager.listeningShards {
		if isListening {
			listeningShards = append(listeningShards, shardID)
		}
	}
	return listeningShards
}

func (n *Node) RemoveListeningShards(shardID ShardIDType) {
	if _, prs := n.ShardManager.listeningShards[shardID]; prs {
		// s.listeningShards[shardID] = false
		delete(n.ShardManager.listeningShards, shardID)
	}
	if _, prs := n.ShardProtocols[shardID]; prs {
		// s.listeningShards[shardID] = false
		delete(n.ShardProtocols, shardID)
	}
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
	return true
}
