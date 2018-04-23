package main

import (
	"testing"
)

func makeTestingNode(number int) (*Node, error) {
	listeningPort := number + 10000
	return makeNode(listeningPort, int64(number))
}

func TestListeningShards(t *testing.T) {
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
	node.UnlistenShard(testingShardID) // ensure no side effect
}

// func TestListeningShards(t *testing.T) {
// }

func makePeerNodes(t *testing.T) (*Node, *Node) {
	node0, err := makeTestingNode(0)
	if err != nil {
		t.Error("Failed to create node")
	}
	node1, err := makeTestingNode(1)
	if err != nil {
		t.Error("Failed to create node")
	}
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
}
