package main

import (
	"bufio"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/Qma7H6RW8wRrfZpNSXwxYGcd1E149s42FpWNpDNieSVrnU/go-libp2p-peer"
	host "gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// node client version
const clientVersion = "go-p2p-node/0.0.1"

type ShardIDType int64

type ListeningShards struct {
	shardMap map[ShardIDType]bool
}

func NewListeningShards() *ListeningShards {
	return &ListeningShards{
		shardMap: make(map[ShardIDType]bool, numShards),
	}
}

func (ls *ListeningShards) Add(shardID ShardIDType) {
	ls.shardMap[shardID] = true
}

func (ls *ListeningShards) ToSlice() []ShardIDType {
	shards := []ShardIDType{}
	for shardID, status := range ls.shardMap {
		if status {
			shards = append(shards, shardID)
		}
	}
	return shards
}

func (ls *ListeningShards) Remove(shardID ShardIDType) {
	if _, prs := ls.shardMap[shardID]; prs {
		// s.listeningShards[shardID] = false
		delete(ls.shardMap, shardID)
	}
}

type ShardManager struct {
	listeningShards *ListeningShards
	PeerShards      map[peer.ID]([]*ListeningShards)
}

// Node type - a p2p host implementing one or more p2p protocols
type Node struct {
	host.Host        // lib-p2p host
	*AddPeerProtocol // addpeer protocol impl

	// Shard related
	ShardManager
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
	n.listeningShards = NewListeningShards()
}

func (n *Node) ListenShard(shardID ShardIDType) {
	n.listeningShards.Add(shardID)
	n.ShardProtocols[shardID] = NewShardProtocol(n, shardID)
}

func (n *Node) UnlistenShard(shardID ShardIDType) {
	n.listeningShards.Remove(shardID)
	if _, prs := n.ShardProtocols[shardID]; prs {
		// s.listeningShards[shardID] = false
		delete(n.ShardProtocols, shardID)
	}
}

func (n *Node) ListListeningShards() []ShardIDType {
	return n.listeningShards.ToSlice()
}

func (n *Node) AddPeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	// TODO
}

func (n *Node) RemovePeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	// TODO
}

func (n *Node) GetPeerListeningShard(peerID peer.ID) {
	// TODO
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
