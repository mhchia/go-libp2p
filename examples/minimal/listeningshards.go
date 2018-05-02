package main

import (
	"context"
	"encoding/json"
	"log"
	"sort"

	floodsub "github.com/libp2p/go-floodsub"
	peer "github.com/libp2p/go-libp2p-peer"
)

const listeningShardTopic = "listeningShardTopic"

type ShardIDType = int64

type ListeningShards struct {
	shardMap map[ShardIDType]bool
}

func NewListeningShards() *ListeningShards {
	return &ListeningShards{
		shardMap: make(map[ShardIDType]bool, numShards),
	}
}

func ListeningShardsFromSlice(shards []ShardIDType) *ListeningShards {
	listeningShards := NewListeningShards()
	for _, shardId := range shards {
		listeningShards.Add(shardId)
	}
	return listeningShards
}

func ListeningShardsFromBytes(bytes []byte) *ListeningShards {
	shardSlice := []ShardIDType{}
	err := json.Unmarshal(bytes, &shardSlice)
	if err != nil {
		log.Fatal("error: ", err)
	}
	return ListeningShardsFromSlice(shardSlice)
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
	sort.Slice(shards, func(i, j int) bool { return shards[i] < shards[j] })
	return shards
}

func (ls *ListeningShards) ToBytes() []byte {
	bytes, err := json.Marshal(ls.ToSlice())
	if err != nil {
		log.Fatal("error: ", err)
	}
	return bytes
}

func (ls *ListeningShards) Remove(shardID ShardIDType) {
	if _, prs := ls.shardMap[shardID]; prs {
		// s.listeningShards[shardID] = false
		delete(ls.shardMap, shardID)
	}
}

type ShardManager struct {
	node *Node // local host

	Floodsub *floodsub.PubSub
	sub      *floodsub.Subscription

	peerListeningShards map[peer.ID]*ListeningShards // TODO: handle the case when peer leave
	done                chan bool
}

func NewShardManager(node *Node) *ShardManager {
	ctx := context.Background()
	service, err := floodsub.NewFloodSub(ctx, node.Host)
	if err != nil {
		log.Fatalln(err)
	}
	p := &ShardManager{
		node:                node,
		Floodsub:            service,
		sub:                 SubscribeShardNotifications(service),
		peerListeningShards: make(map[peer.ID]*ListeningShards),
		done:                make(chan bool),
	}
	p.ListenShardNotifications()
	return p
}

// Listening shard

func (n *ShardManager) AddPeerListeningShard(peerID peer.ID, shardID ShardIDType) {
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

func (n *ShardManager) RemovePeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	if !n.IsPeerListeningShard(peerID, shardID) {
		return
	}
	n.peerListeningShards[peerID].Remove(shardID)
}

func (n *ShardManager) GetPeerListeningShard(peerID peer.ID) []ShardIDType {
	if _, prs := n.peerListeningShards[peerID]; !prs {
		return make([]ShardIDType, 0)
	}
	return n.peerListeningShards[peerID].ToSlice()
}

func (n *ShardManager) SetPeerListeningShard(peerID peer.ID, shardIDs []ShardIDType) {
	listeningShards := n.GetPeerListeningShard(peerID)
	for _, shardID := range listeningShards {
		n.RemovePeerListeningShard(peerID, shardID)
	}
	for _, shardID := range shardIDs {
		n.AddPeerListeningShard(peerID, shardID)
	}
}

func (n *ShardManager) IsPeerListeningShard(peerID peer.ID, shardID ShardIDType) bool {
	if _, prs := n.peerListeningShards[peerID]; !prs {
		return false
	}
	shards := n.GetPeerListeningShard(peerID)
	return InShards(shardID, shards)
}

// PubSub related

func (n *ShardManager) ListenShardNotifications() {
	ctx := context.Background()
	go func() {
		if n.sub == nil {
			return
		}
		msg, err := n.sub.Next(ctx)
		if err != nil {
			log.Fatal(err)
		}
		peerID := msg.GetFrom()
		// TODO: maybe should check if `peerID` is the node itself
		listeningShards := ListeningShardsFromBytes(msg.GetData())
		n.SetPeerListeningShard(peerID, listeningShards.ToSlice())
		log.Printf(
			"%v: receive: peerID=%v, listeningShards=%v",
			n.node.ID(),
			peerID,
			listeningShards,
		)
	}()
}

func SubscribeShardNotifications(pubsub *floodsub.PubSub) *floodsub.Subscription {
	sub, err := pubsub.Subscribe(listeningShardTopic)
	if err != nil {
		log.Fatal(err)
	}
	return sub
}

func (n *ShardManager) UnsubscribeShardNotifications() {
	n.sub = nil
}

func (n *ShardManager) NotifyListeningShards(listeningShards *ListeningShards) {
	bytes := listeningShards.ToBytes()
	n.Floodsub.Publish(listeningShardTopic, bytes)
}
