package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
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

// TODO: need checks against the format of bytes
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

	Floodsub            *floodsub.PubSub
	listeningShardsSub  *floodsub.Subscription
	shardCollationsSubs map[ShardIDType]*floodsub.Subscription
	collations          map[string]struct{}
	lock                sync.Mutex

	peerListeningShards map[peer.ID]*ListeningShards // TODO: handle the case when peer leave
}

const collationTopicFmt = "shardCollations_%d"

func getCollationsTopic(shardID ShardIDType) string {
	return fmt.Sprintf(collationTopicFmt, shardID)
}

func NewShardManager(ctx context.Context, node *Node) *ShardManager {
	service, err := floodsub.NewFloodSub(ctx, node.Host)
	if err != nil {
		log.Fatalln(err)
	}
	p := &ShardManager{
		node:                node,
		Floodsub:            service,
		listeningShardsSub:  nil,
		shardCollationsSubs: make(map[ShardIDType]*floodsub.Subscription),
		collations:          make(map[string]struct{}),
		lock:                sync.Mutex{},
		peerListeningShards: make(map[peer.ID]*ListeningShards),
	}
	p.SubscribeListeningShards()
	p.ListenListeningShards(ctx)
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

func (n *ShardManager) ListenSubs(ctx context.Context) {
	// TODO: handle all subs with `reflect.select` here
}

// listeningShards notification

func (n *ShardManager) ListenListeningShards(ctx context.Context) {
	// this is necessary, because n.listeningShardsSub might be set to `nil`
	// after `UnsubscribeListeningShards`
	listeningShardsSub := n.listeningShardsSub
	go func() {
		for {
			msg, err := listeningShardsSub.Next(ctx)
			if err != nil {
				// Will enter here if
				// 	1. `sub.Cancel()` is called with
				//		err="subscription cancelled by calling sub.Cancel()"
				// 	2. ctx is cancelled with err="context canceled"
				// log.Print("ListenListeningShards: ", err)
				return
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
	n.listeningShardsSub.Cancel()
	n.listeningShardsSub = nil
}

func (n *ShardManager) PublishListeningShards(listeningShards *ListeningShards) {
	bytes := listeningShards.ToBytes()
	n.Floodsub.Publish(listeningShardTopic, bytes)
}

// shard collations

func Hash(msg *pbmsg.Collation) string {
	dataInBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error occurs when hashing %v", msg)
	}
	return string(gethcrypto.Keccak256(dataInBytes))
}

func (n *ShardManager) ListenShardCollations(shardID ShardIDType) {
	if !n.IsShardCollationsSubscribed(shardID) {
		return
	}
	shardCollationsSub := n.shardCollationsSubs[shardID]
	go func() {
		for {
			// TODO: consider to pass the context from outside?
			msg, err := shardCollationsSub.Next(context.Background())
			if err != nil {
				// log.Print(err)
				return
			}
			bytes := msg.GetData()
			collation := pbmsg.Collation{}
			err = proto.Unmarshal(bytes, &collation)
			if err != nil {
				// log.Fatal(err)
				continue
			}
			// TODO: need some check against collations
			// TODO: do something(store the hash?)
			collationHash := Hash(&collation)
			n.lock.Lock()
			n.collations[collationHash] = struct{}{}
			log.Printf("!@# current numCollations=%d", len(n.collations))
			n.lock.Unlock()
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
	_, prs := n.shardCollationsSubs[shardID]
	return prs && (n.shardCollationsSubs[shardID] != nil)
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
	n.shardCollationsSubs[shardID] = collationsSub
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
	// TODO: unsubscribe in pubsub
	if n.IsShardCollationsSubscribed(shardID) {
		n.shardCollationsSubs[shardID].Cancel()
		n.shardCollationsSubs[shardID] = nil
	}
}
