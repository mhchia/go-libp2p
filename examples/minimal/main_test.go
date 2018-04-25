package main

import (
	"context"
	peer "gx/ipfs/Qma7H6RW8wRrfZpNSXwxYGcd1E149s42FpWNpDNieSVrnU/go-libp2p-peer"
	"log"
	"testing"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"
)

func makeTestingNode(number int) (*Node, error) {
	listeningPort := number + 10000
	return makeNode(listeningPort, int64(number))
}

/* unit tests */
func TestListeningShards(t *testing.T) {
	// TODO: add check for `ShardProtocol`
	node, err := makeTestingNode(0)
	if err != nil {
		t.Error("Failed to create node")
	}
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
	node, err := makeTestingNode(0)
	if err != nil {
		t.Error("Failed to create node")
	}
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
			"Peer %v should be able to listen to shardID bigger than %v",
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

func makePeerNodes(t *testing.T) (*Node, *Node) {
	node0, err := makeTestingNode(0)
	if err != nil {
		t.Error("Failed to create node")
	}
	node1, err := makeTestingNode(1)
	if err != nil {
		t.Error("Failed to create node")
	}
	// if node0.IsPeer(node1.ID()) || node1.IsPeer(node0.ID()) {
	// 	t.Error("Two initial nodes should not be connected without `AddPeer`")
	// }
	node0.AddPeer(node1.GetFullAddr())
	// wait until node0 receive the response from node1
	<-node0.AddPeerProtocol.done
	if !node0.IsPeer(node1.ID()) || !node1.IsPeer(node0.ID()) {
		t.Error("Failed to add peer")
	}
	return node0, node1
}

// TODO: Need to think about the case with bootstrap nodes
func TestAddPeer(t *testing.T) {
	makePeerNodes(t)
}

func TestNotifyShards(t *testing.T) {
	// node0, node1 := makePeerNodes(t)
	node0, node1 := makePeerNodes(t)
	node0ListeningShards := []ShardIDType{12, 34, 56}
	node0.NotifyShards(node1.ID(), node0ListeningShards)
	<-node1.NotifyShardsProtocol.done
	for _, shardID := range node0ListeningShards {
		if !node1.IsPeerListeningShard(node0.ID(), shardID) {
			t.Errorf(
				"Peer %v should be conscious that %v is listening to shard %v",
				node1.ID(),
				node0.ID(),
				shardID,
			)
		}
	}
	// send again with different shards
	nowNode0ListeningShards := []ShardIDType{4, 5}
	node0.NotifyShards(node1.ID(), nowNode0ListeningShards)
	<-node1.NotifyShardsProtocol.done
	for _, shardID := range nowNode0ListeningShards {
		if !node1.IsPeerListeningShard(node0.ID(), shardID) {
			t.Errorf(
				"Peer %v should be conscious that %v is listening to shard %v",
				node1.ID(),
				node0.ID(),
				shardID,
			)
		}
	}
}

func TestSendCollation(t *testing.T) {
	node0, node1 := makePeerNodes(t)
	var testingShardID int64 = 42
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
	var notlistenedShardID int64 = 24
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
