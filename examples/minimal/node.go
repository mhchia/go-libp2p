package main

import (
	"bufio"
	"fmt"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	"gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	peer "gx/ipfs/Qma7H6RW8wRrfZpNSXwxYGcd1E149s42FpWNpDNieSVrnU/go-libp2p-peer"
	host "gx/ipfs/QmfCtHMCd9xFvehvHeVxtKVXJTMVTuHhyPRVHEXetn87vL/go-libp2p-host"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// node client version
const clientVersion = "go-p2p-node/0.0.1"

type ShardIDType uint64

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
	peerListeningShards map[peer.ID]*ListeningShards // TODO: handle the case when peer leave
}

// Node type - a p2p host implementing one or more p2p protocols
type Node struct {
	host.Host        // lib-p2p host
	*AddPeerProtocol // addpeer protocol impl

	// Shard related
	// TODO: maybe move all sharding related things to `ShardManager`?
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
	n.peerListeningShards = make(map[peer.ID]*ListeningShards)
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
	}
}

func (n *Node) GetListeningShards() []ShardIDType {
	return n.GetPeerListeningShard(n.ID())
}

func inShards(shardID ShardIDType, shards []ShardIDType) bool {
	for _, value := range shards {
		if value == shardID {
			return true
		}
	}
	return false
}

func (n *Node) IsShardListened(shardID ShardIDType) bool {
	return inShards(shardID, n.GetListeningShards())
}

func (n *Node) AddPeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	if shardID >= numShards {
		return
	}
	if n.IsPeerListeningShard(peerID, shardID) {
		return
	}
	if _, prs := n.peerListeningShards[peerID]; !prs {
		n.peerListeningShards[peerID] = NewListeningShards()
	}
	n.peerListeningShards[peerID].Add(shardID)
}

func (n *Node) RemovePeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	if !n.IsPeerListeningShard(peerID, shardID) {
		return
	}
	n.peerListeningShards[peerID].Remove(shardID)
}

func (n *Node) GetPeerListeningShard(peerID peer.ID) []ShardIDType {
	if _, prs := n.peerListeningShards[peerID]; !prs {
		return make([]ShardIDType, 0)
	}
	return n.peerListeningShards[peerID].ToSlice()
}

func (n *Node) IsPeerListeningShard(peerID peer.ID, shardID ShardIDType) bool {
	if _, prs := n.peerListeningShards[peerID]; !prs {
		return false
	}
	shards := n.GetPeerListeningShard(peerID)
	return inShards(shardID, shards)
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

func (n *Node) GetFullAddr() string {
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", n.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := n.Addrs()[0]
	log.Printf("addr=%s", addr)
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
