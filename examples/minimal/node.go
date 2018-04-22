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

// Node type - a p2p host implementing one or more p2p protocols
type Node struct {
	host.Host        // lib-p2p host
	*AddPeerProtocol // ping protocol impl
	ShardProtocols   map[int64]*ShardProtocol
	// add other protocols here...
}

// Create a new node with its implemented protocols
func NewNode(host host.Host) *Node {
	node := &Node{Host: host}
	node.AddPeerProtocol = NewAddPeerProtocol(node)
	var testShardID int64 = 87
	node.ShardProtocols = make(map[int64]*ShardProtocol, 100)
	node.ShardProtocols[testShardID] = NewShardProtocol(node, testShardID)
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
	return true
}
