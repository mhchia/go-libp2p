package main

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/protobuf/proto"
	floodsub "github.com/libp2p/go-floodsub"
	peer "github.com/libp2p/go-libp2p-peer"
	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"
)

const listeningShardTopic = "listeningShard"

type ShardIDType = int64

type ListeningShards struct {
	shardBits []byte
}

const byteSize = 8 // in bits

func shardIDToBitIndex(shardID ShardIDType) (byte, byte, error) {
	if shardID >= numShards {
		return 0, 0, fmt.Errorf("Wrong shardID %v", shardID)
	}
	byteIndex := byte(shardID / byteSize)
	bitIndex := byte(shardID % byteSize)
	return byteIndex, bitIndex, nil
}

func (ls *ListeningShards) unsetShard(shardID ShardIDType) error {
	byteIndex, bitIndex, err := shardIDToBitIndex(shardID)
	if err != nil {
		return fmt.Errorf("")
	}
	if int(byteIndex) >= len(ls.shardBits) {
		return fmt.Errorf(
			"(byteIndex=%v) >= (len(shardBits)=%v)",
			byteIndex,
			len(ls.shardBits),
		)
	}
	// log.Printf("shardID=%v, byteIndex=%v, bitIndex=%v", shardID, byteIndex, bitIndex)
	ls.shardBits[byteIndex] &= (^(1 << bitIndex))
	return nil
}

func (ls *ListeningShards) setShard(shardID ShardIDType) error {
	byteIndex, bitIndex, err := shardIDToBitIndex(shardID)
	if err != nil {
		return fmt.Errorf("")
	}
	if int(byteIndex) >= len(ls.shardBits) {
		return fmt.Errorf(
			"(byteIndex=%v) >= (len(shardBits)=%v)",
			byteIndex,
			len(ls.shardBits),
		)
	}
	// log.Printf("shardID=%v, byteIndex=%v, bitIndex=%v", shardID, byteIndex, bitIndex)
	ls.shardBits[byteIndex] |= (1 << bitIndex)
	return nil
}

func (ls *ListeningShards) isShardSet(shardID ShardIDType) bool {
	byteIndex, bitIndex, err := shardIDToBitIndex(shardID)
	if err != nil {
		fmt.Errorf("")
	}
	index := ls.shardBits[byteIndex] & (1 << bitIndex)
	return index != 0
}

func (ls *ListeningShards) getShards() []ShardIDType {
	shards := []ShardIDType{}
	for shardID := ShardIDType(0); shardID < numShards; shardID++ {
		if ls.isShardSet(shardID) {
			shards = append(shards, shardID)
		}
	}
	return shards
}

func NewListeningShards() *ListeningShards {
	return &ListeningShards{
		shardBits: make([]byte, (numShards/8)+1),
	}
}

func ListeningShardsFromSlice(shards []ShardIDType) *ListeningShards {
	listeningShards := NewListeningShards()
	for _, shardID := range shards {
		listeningShards.setShard(shardID)
	}
	return listeningShards
}

func ListeningShardsFromBytes(bytes []byte) *ListeningShards {
	listeningShards := NewListeningShards()
	listeningShards.shardBits = bytes
	return listeningShards
}

func (ls *ListeningShards) ToBytes() []byte {
	return ls.shardBits
}

type ShardManager struct {
	node *Node // local host

	Floodsub           *floodsub.PubSub
	listeningShardsSub *floodsub.Subscription
	collationSubs      map[ShardIDType]*floodsub.Subscription

	peerListeningShards map[peer.ID]*ListeningShards // TODO: handle the case when peer leave
}

const collationTopicFmt = "shardCollations_%d"

func getCollationsTopic(shardID ShardIDType) string {
	return fmt.Sprintf(collationTopicFmt, shardID)
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
		listeningShardsSub:  nil,
		collationSubs:       make(map[ShardIDType]*floodsub.Subscription),
		peerListeningShards: make(map[peer.ID]*ListeningShards),
	}
	p.SubscribeListeningShards()
	p.ListenListeningShards()
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
	n.peerListeningShards[peerID].setShard(shardID)
}

func (n *ShardManager) RemovePeerListeningShard(peerID peer.ID, shardID ShardIDType) {
	if !n.IsPeerListeningShard(peerID, shardID) {
		return
	}
	n.peerListeningShards[peerID].unsetShard(shardID)
}

func (n *ShardManager) GetPeerListeningShard(peerID peer.ID) []ShardIDType {
	if _, prs := n.peerListeningShards[peerID]; !prs {
		return make([]ShardIDType, 0)
	}
	return n.peerListeningShards[peerID].getShards()
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

func (n *ShardManager) GetNodesInShard(shardID ShardIDType) []peer.ID {
	peers := []peer.ID{}
	for peerID, listeningShards := range n.peerListeningShards {
		if listeningShards.isShardSet(shardID) {
			peers = append(peers, peerID)
		}
	}
	return peers
}

func (n *ShardManager) GetPeersInShard(shardID ShardIDType) []peer.ID {
	peers := []peer.ID{}
	for _, peerID := range n.node.Peerstore().Peers() {
		if n.IsPeerListeningShard(peerID, shardID) {
			peers = append(peers, peerID)
		}
	}
	return peers
}

//
// PubSub related
//

// listeningShards notification

func (n *ShardManager) ListenListeningShards() {
	ctx := context.Background()
	go func() {
		for {
			if n.listeningShardsSub == nil {
				return
			}
			msg, err := n.listeningShardsSub.Next(ctx)
			if err != nil {
				log.Fatal(err)
			}
			peerID := msg.GetFrom()
			// TODO: maybe should check if `peerID` is the node itself
			listeningShards := ListeningShardsFromBytes(msg.GetData())
			n.SetPeerListeningShard(peerID, listeningShards.getShards())
			log.Printf(
				"%v: receive: peerID=%v, listeningShards=%v",
				n.node.ID(),
				peerID,
				listeningShards,
			)
		}
	}()
}

func (n *ShardManager) SubscribeListeningShards() {
	listeningShardsSub, err := n.Floodsub.Subscribe(listeningShardTopic)
	if err != nil {
		log.Fatal(err)
	}
	n.listeningShardsSub = listeningShardsSub
}

func (n *ShardManager) UnsubscribeListeningShards() {
	n.listeningShardsSub = nil
}

func (n *ShardManager) PublishListeningShards(listeningShards *ListeningShards) {
	bytes := listeningShards.ToBytes()
	n.Floodsub.Publish(listeningShardTopic, bytes)
}

// shard collations

func (n *ShardManager) ListenShardCollations(shardID ShardIDType) {
	ctx := context.Background()
	go func() {
		for {
			if !n.IsShardCollationsSubscribed(shardID) {
				return
			}
			msg, err := n.collationSubs[shardID].Next(ctx)
			if err != nil {
				log.Fatal(err)
			}
			bytes := msg.GetData()
			collation := pbmsg.Collation{}
			err = proto.Unmarshal(bytes, &collation)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf(
				"%v: receive: collation: shardId=%v, number=%v, blobs=%v",
				n.node.ID(),
				collation.GetShardID(),
				collation.GetNumber(),
				collation.GetBlobs(),
			)
		}
	}()
}

func (n *ShardManager) IsShardCollationsSubscribed(shardID ShardIDType) bool {
	_, prs := n.collationSubs[shardID]
	return prs && (n.collationSubs[shardID] != nil)
}

func (n *ShardManager) SubscribeShardCollations(shardID ShardIDType) {
	if n.IsShardCollationsSubscribed(shardID) {
		return
	}
	collationsTopic := getCollationsTopic(shardID)
	collationsSub, err := n.Floodsub.Subscribe(collationsTopic)
	if err != nil {
		log.Fatal(err)
	}
	n.collationSubs[shardID] = collationsSub
	log.Printf("Subscribed shard %v", shardID)
}

func (n *ShardManager) sendCollation(shardID ShardIDType, number int64, blobs string) bool {
	if !n.IsShardCollationsSubscribed(shardID) {
		log.Fatalf("Attempted to send collation in a not subscribed shard %v", shardID)
		return false
	}
	// create message data
	data := &pbmsg.Collation{
		ShardID: shardID,
		Number:  number,
		Blobs:   blobs,
	}

	return n.sendCollationMessage(data)
}

func (n *ShardManager) sendCollationMessage(collation *pbmsg.Collation) bool {
	if !n.IsShardCollationsSubscribed(collation.GetShardID()) {
		return false
	}
	// bytes := listeningShards.ToBytes()
	collationsTopic := getCollationsTopic(collation.ShardID)
	bytes, err := proto.Marshal(collation)
	if err != nil {
		log.Fatal(err)
		return false
	}
	err = n.Floodsub.Publish(collationsTopic, bytes)
	if err != nil {
		log.Fatal(err)
		return false
	}
	return true
}

func (n *ShardManager) UnsubscribeShardCollations(shardID ShardIDType) {
	if n.IsShardCollationsSubscribed(shardID) {
		n.collationSubs[shardID] = nil
	}
}
