package main

import (
	"context"
	"log"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	gologging "github.com/whyrusleeping/go-logging"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"
)

func makeUnbootstrappedNode(t *testing.T, number int) *Node {
	return makeTestingNode(t, number, []pstore.PeerInfo{})
}

func makeTestingNode(t *testing.T, number int, bootstrapPeers []pstore.PeerInfo) *Node {
	listeningPort := number + 10000
	node, err := makeNode(listeningPort, int64(number), bootstrapPeers)
	if err != nil {
		t.Error("Failed to create node")
	}
	return node
}

/* unit tests */

func TestListeningShards(t *testing.T) {
	ls := NewListeningShards()
	ls.setShard(1)
	lsSlice := ls.getShards()
	if (len(lsSlice) != 1) || lsSlice[0] != ShardIDType(1) {
		t.Error()
	}
	ls.setShard(42)
	if len(ls.getShards()) != 2 {
		t.Error()
	}
	// test `ToBytes` and `ListeningShardsFromBytes`
	bytes := ls.ToBytes()
	lsNew := ListeningShardsFromBytes(bytes)
	if len(ls.getShards()) != len(lsNew.getShards()) {
		t.Error()
	}
	for index, value := range ls.getShards() {
		if value != lsNew.getShards()[index] {
			t.Error()
		}
	}
}

func TestNodeListeningShards(t *testing.T) {
	// TODO: add check for `ShardProtocol`
	node := makeUnbootstrappedNode(t, 0)
	var testingShardID ShardIDType = 87
	// test `IsShardListened`
	if node.IsShardListened(testingShardID) {
		t.Errorf("Shard %v haven't been listened", testingShardID)
	}
	// test `ListenShard`
	node.ListenShard(testingShardID)
	if !node.IsShardListened(testingShardID) {
		t.Errorf("Shard %v should have been listened", testingShardID)
	}
	if _, prs := node.ShardProtocols[testingShardID]; !prs {
		t.Errorf("ShardProtocol[%v] should exist", testingShardID)
	}
	anotherShardID := testingShardID + 1
	node.ListenShard(anotherShardID)
	if !node.IsShardListened(anotherShardID) {
		t.Errorf("Shard %v should have been listened", anotherShardID)
	}
	shardIDs := node.GetListeningShards()
	if len(shardIDs) != 2 {
		t.Errorf("We should have 2 shards being listened, instead of %v", len(shardIDs))
	}
	// test `UnlistenShard`
	node.UnlistenShard(testingShardID)
	if node.IsShardListened(testingShardID) {
		t.Errorf("Shard %v should have been unlistened", testingShardID)
	}
	if _, prs := node.ShardProtocols[testingShardID]; prs {
		t.Errorf("ShardProtocol[%v] should exist", testingShardID)
	}
	node.UnlistenShard(testingShardID) // ensure no side effect
}

func TestPeerListeningShards(t *testing.T) {
	node := makeUnbootstrappedNode(t, 0)
	arbitraryPeerID := peer.ID("123456")
	if node.IsPeer(arbitraryPeerID) {
		t.Errorf(
			"PeerID %v should be a non-peer peerID to make the test work correctly",
			arbitraryPeerID,
		)
	}
	var testingShardID ShardIDType = 87
	if len(node.GetPeerListeningShard(arbitraryPeerID)) != 0 {
		t.Errorf("Peer %v should not be listening to any shard", arbitraryPeerID)
	}
	if node.IsPeerListeningShard(arbitraryPeerID, testingShardID) {
		t.Errorf("Peer %v should not be listening to shard %v", arbitraryPeerID, testingShardID)
	}
	node.AddPeerListeningShard(arbitraryPeerID, testingShardID)
	if !node.IsPeerListeningShard(arbitraryPeerID, testingShardID) {
		t.Errorf("Peer %v should be listening to shard %v", arbitraryPeerID, testingShardID)
	}
	node.AddPeerListeningShard(arbitraryPeerID, numShards)
	if node.IsPeerListeningShard(arbitraryPeerID, numShards) {
		t.Errorf(
			"Peer %v should not be able to listen to shardID bigger than %v",
			arbitraryPeerID,
			numShards,
		)
	}
	// listen to multiple shards
	anotherShardID := testingShardID + 1 // notice that it should be less than `numShards`
	node.AddPeerListeningShard(arbitraryPeerID, anotherShardID)
	if !node.IsPeerListeningShard(arbitraryPeerID, anotherShardID) {
		t.Errorf("Peer %v should be listening to shard %v", arbitraryPeerID, testingShardID)
	}
	if len(node.GetPeerListeningShard(arbitraryPeerID)) != 2 {
		t.Errorf(
			"Peer %v should be listening to %v shards, not %v",
			arbitraryPeerID,
			2,
			len(node.GetPeerListeningShard(arbitraryPeerID)),
		)
	}
	node.RemovePeerListeningShard(arbitraryPeerID, anotherShardID)
	if node.IsPeerListeningShard(arbitraryPeerID, anotherShardID) {
		t.Errorf("Peer %v should be listening to shard %v", arbitraryPeerID, testingShardID)
	}
	if len(node.GetPeerListeningShard(arbitraryPeerID)) != 1 {
		t.Errorf(
			"Peer %v should be only listening to %v shards, not %v",
			arbitraryPeerID,
			1,
			len(node.GetPeerListeningShard(arbitraryPeerID)),
		)
	}

	// see if it is still correct with multiple peers
	anotherPeerID := peer.ID("9547")
	if node.IsPeer(anotherPeerID) {
		t.Errorf(
			"PeerID %v should be a non-peer peerID to make the test work correctly",
			anotherPeerID,
		)
	}
	if len(node.GetPeerListeningShard(anotherPeerID)) != 0 {
		t.Errorf("Peer %v should not be listening to any shard", anotherPeerID)
	}
	node.AddPeerListeningShard(anotherPeerID, testingShardID)
	if len(node.GetPeerListeningShard(anotherPeerID)) != 1 {
		t.Errorf("Peer %v should be listening to 1 shard", anotherPeerID)
	}
	// make sure not affect other peers
	if len(node.GetPeerListeningShard(arbitraryPeerID)) != 1 {
		t.Errorf("Peer %v should be listening to 1 shard", arbitraryPeerID)
	}
	node.RemovePeerListeningShard(anotherPeerID, testingShardID)
	if len(node.GetPeerListeningShard(anotherPeerID)) != 0 {
		t.Errorf("Peer %v should be listening to 0 shard", anotherPeerID)
	}
	if len(node.GetPeerListeningShard(arbitraryPeerID)) != 1 {
		t.Errorf("Peer %v should be listening to 1 shard", arbitraryPeerID)
	}
}

func connect(t *testing.T, a, b host.Host) {
	pinfo := a.Peerstore().PeerInfo(a.ID())
	err := b.Connect(context.Background(), pinfo)
	if err != nil {
		t.Fatal(err)
	}
}

func makeNodes(t *testing.T, number int) []*Node {
	nodes := make([]*Node, number)
	for i := 0; i < number; i++ {
		nodes = append(nodes, makeUnbootstrappedNode(t, i))
	}
	return nodes
}

func makePeerNodes(t *testing.T) (*Node, *Node) {
	node0 := makeUnbootstrappedNode(t, 0)
	node1 := makeUnbootstrappedNode(t, 1)
	// if node0.IsPeer(node1.ID()) || node1.IsPeer(node0.ID()) {
	// 	t.Error("Two initial nodes should not be connected without `AddPeer`")
	// }
	node0.AddPeer(node1.GetFullAddr())
	// wait until node0 receive the response from node1
	<-node0.AddPeerProtocol.done
	if !node0.IsPeer(node1.ID()) || !node1.IsPeer(node0.ID()) {
		t.Error("Failed to add peer")
	}
	connect(t, node0, node1)
	return node0, node1
}

func TestAddPeer(t *testing.T) {
	makePeerNodes(t)
}

func TestSendCollation(t *testing.T) {
	node0, node1 := makePeerNodes(t)
	var testingShardID ShardIDType = 42
	node0.ListenShard(testingShardID)
	// fail
	node0.ShardProtocols[testingShardID].sendCollation(
		node1.ID(),
		1,
		"123",
	)

	node1.ListenShard(testingShardID)
	// fail: if the collation's shardID does not correspond to the protocol's shardID,
	//		 receiver should reject it
	var notlistenedShardID ShardIDType = 24
	req := &pbmsg.SendCollationRequest{
		ShardID: notlistenedShardID,
		Number:  1,
		Blobs:   "123",
	}
	s, err := node0.NewStream(
		context.Background(),
		node1.ID(),
		getSendCollationRequestProtocolID(testingShardID),
	)
	if err != nil {
		log.Println(err)
	}
	node0.sendProtoMessage(req, s)
	if result := <-node1.ShardProtocols[testingShardID].done; result {
		t.Error("node1 should consider this message wrong")
	}

	// success
	node0.ShardProtocols[testingShardID].sendCollation(
		node1.ID(),
		1,
		"123",
	)
	if result := <-node1.ShardProtocols[testingShardID].done; !result {
		t.Error("node1 should consider this message wrong")
	}
}

func makePartiallyConnected3Nodes(t *testing.T) []*Node {
	node0, node1 := makePeerNodes(t)
	node2 := makeUnbootstrappedNode(t, 2)
	node2.AddPeer(node1.GetFullAddr())
	connect(t, node1, node2)
	return [](*Node){node0, node1, node2}
}

func TestRouting(t *testing.T) {
	// set the logger to DEBUG, to see the process of dht.FindPeer
	// we should be able to see something like
	// "dht: FindPeer <peer.ID d3wzD2> true routed.go:76", if successfully found the desire peer
	node0 := makeUnbootstrappedNode(t, 0)
	node1 := makeUnbootstrappedNode(t, 1)
	node2 := makeUnbootstrappedNode(t, 2)
	node1.AddPeer(node0.GetFullAddr())
	var testingShardID ShardIDType = 12
	node0.ListenShard(testingShardID)
	node1.ListenShard(testingShardID)
	node2.ListenShard(testingShardID)
	req := &pbmsg.SendCollationRequest{
		ShardID: testingShardID,
		Number:  1,
		Blobs:   "123",
	}
	collationHash := Hash(req)
	node1.ShardProtocols[testingShardID].sendCollationMessage(node2.ID(), req)
	time.Sleep(time.Millisecond * 100)
	if _, prs := node2.ShardProtocols[testingShardID].receivedCollations[collationHash]; prs {
		t.Error("node1 should not be able to reach node2")
	}
	// node1 <-> node0 <->node2
	node0.AddPeer(node2.GetFullAddr())
	if node1.IsPeer(node2.ID()) {
		t.Error("node1 should not be able to reach node2 before routing")
	}
	node1.ShardProtocols[testingShardID].sendCollationMessage(node2.ID(), req)
	<-node2.ShardProtocols[testingShardID].done
	if _, prs := node2.ShardProtocols[testingShardID].receivedCollations[collationHash]; !prs {
		t.Error("node1 should be able to reach node2 now")
	}
	if !node1.IsPeer(node2.ID()) {
		t.Error("node1 should be able a peer of node2 now")
	}
}

func TestPubSub(t *testing.T) {
	ctx := context.Background()

	nodes := makePartiallyConnected3Nodes(t)

	topic := "interestedShards"

	subch0, err := nodes[0].Floodsub.Subscribe(topic)
	if err != nil {
		t.Fatal(err)
	}

	// TODO: This sleep is necessary!!! Find out why
	time.Sleep(time.Millisecond * 100)

	publishMsg := "789"
	err = nodes[1].Floodsub.Publish(topic, []byte(publishMsg))
	if err != nil {
		t.Fatal(err)
	}
	msg0, err := subch0.Next(ctx)
	if string(msg0.Data) != publishMsg {
		t.Fatal(err)
	}
	if msg0.GetFrom() != nodes[1].ID() {
		t.Error("Wrong ID")
	}

}

func TestPubSubNotifyListeningShards(t *testing.T) {
	nodes := makePartiallyConnected3Nodes(t)

	listeningShards := NewListeningShards()
	// ensure notifyShards message is propagated through node1
	listeningShards.setShard(42)
	if len(nodes[1].GetPeerListeningShard(nodes[0].ID())) != 0 {
		t.Error()
	}
	nodes[0].NotifyListeningShards(listeningShards)
	time.Sleep(time.Millisecond * 100)
	if len(nodes[1].GetPeerListeningShard(nodes[0].ID())) != 1 {
		t.Error()
	}
	if len(nodes[2].GetPeerListeningShard(nodes[0].ID())) != 1 {
		t.Error()
	}
	nodes[1].NotifyListeningShards(listeningShards)
	time.Sleep(time.Millisecond * 100)
	if len(nodes[2].GetPeersInShard(42)) != 2 {
		t.Error()
	}

	// test unsetShard with notifying
	listeningShards.unsetShard(42)
	nodes[0].NotifyListeningShards(listeningShards)
	time.Sleep(time.Millisecond * 100)
	if len(nodes[1].GetPeerListeningShard(nodes[0].ID())) != 0 {
		t.Error()
	}
}

func TestPubSubDuplicateMessages(t *testing.T) {
	golog.SetAllLoggers(gologging.DEBUG) // Change to DEBUG for extra info
	nodes := makePartiallyConnected3Nodes(t)
	nodes = append(nodes, makeTestingNode(t, 3, []pstore.PeerInfo{}))
	connect(t, nodes[0], nodes[3])
	connect(t, nodes[2], nodes[3])

	listeningShards := NewListeningShards()
	listeningShards.setShard(42)
	if len(nodes[1].GetPeerListeningShard(nodes[0].ID())) != 0 {
		t.Error()
	}
	nodes[1].NotifyListeningShards(listeningShards)
	time.Sleep(time.Millisecond * 100)

}

// test if nodes can find each other only with bootnodes
func TestBootstrapIPFS(t *testing.T) {
	golog.SetAllLoggers(gologging.DEBUG)            // Change to DEBUG for extra info
	node0 := makeTestingNode(t, 0, IPFS_PEERS[0:1]) // connect to boostrapping nodes
	node1 := makeTestingNode(t, 1, IPFS_PEERS[0:1]) // connect to local ipfs node
	log.Println("!@#", node0.GetFullAddr())
	log.Println("!@#", node1.GetFullAddr())
	// return
	node0PeerInfo := pstore.PeerInfo{
		ID:    node0.ID(),
		Addrs: []ma.Multiaddr{},
	}
	ctx := context.Background()
	if len(node1.Network().ConnsToPeer(node0.ID())) != 0 {
		t.Error()
	}
	node1.Connect(ctx, node0PeerInfo)
	// testingShardID := ShardIDType(42)
	// node0.ListenShard(testingShardID)
	// node1.ListenShard(testingShardID)
	// // fail
	// node0.ShardProtocols[testingShardID].sendCollation(
	// 	node1.ID(),
	// 	1,
	// 	"123",
	// )
	if len(node1.Network().ConnsToPeer(node0.ID())) == 0 {
		t.Error()
	}
}

func TestSendCollation100Shards(t *testing.T) {
	node0, node1 := makePeerNodes(t)
	blobSize := 1000000
	var numCollations ShardIDType = 1
	for i := ShardIDType(0); i < numShards; i++ {
		node0.ListenShard(i)
		node1.ListenShard(i)
	}
	// time1 := time.Now()
	for i := ShardIDType(0); i < numShards; i++ {
		go func(shardID ShardIDType) {
			for j := ShardIDType(0); j < numCollations; j++ {
				node0.ShardProtocols[shardID].sendCollation(
					node1.ID(),
					j,
					string(make([]byte, blobSize)),
				)
			}
		}(i)
	}
	for i := ShardIDType(0); i < numShards; i++ {
		for j := ShardIDType(0); j < numCollations; j++ {
			<-node1.ShardProtocols[i].done
		}
	}
	// select {}
	log.Println("done")
}
