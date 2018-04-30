package main

import (
	"bufio"
	"context"
	"log"

	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// pattern: /protocol-name/request-or-response-message/version
const notifyShardsRequest = "/notifyShards/request/0.0.1"
const notifyShardsResponse = "/notifyShards/response/0.0.1"

// NotifyShardsProtocol type
type NotifyShardsProtocol struct {
	node *Node     // local host
	done chan bool // only for demo purposes to stop main from terminating
}

func NewNotifyShardsProtocol(node *Node) *NotifyShardsProtocol {
	p := &NotifyShardsProtocol{
		node: node,
		done: make(chan bool),
	}
	node.SetStreamHandler(notifyShardsRequest, p.onRequest)
	// node.SetStreamHandler(notifyShardsResponse, p.onResponse)
	return p
}

// remote peer requests handler
func (p *NotifyShardsProtocol) onRequest(s inet.Stream) {

	// get request data
	data := &pbmsg.NotifyShardsRequest{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		log.Println(err)
		return
	}
	remotePeerID := s.Conn().RemotePeer()
	log.Printf(
		"%s: Received notifyShards request from %s. shardIDs: %v",
		s.Conn().LocalPeer(),
		s.Conn().RemotePeer(),
		data.ShardIDs,
	)
	p.node.SetPeerListeningShard(remotePeerID, data.ShardIDs)
	p.done <- true
	s.Close()
	log.Println("!@#, ", p.node.Network().ConnsToPeer(remotePeerID))
}

func (p *NotifyShardsProtocol) NotifyShards(peerID peer.ID, shardIDs []ShardIDType) bool {
	log.Printf("%s: Sending notifyShards to: %s....", p.node.ID(), peerID)
	// create message data
	req := &pbmsg.NotifyShardsRequest{
		ShardIDs: shardIDs,
	}

	s, err := p.node.NewStream(context.Background(), peerID, notifyShardsRequest)
	if err != nil {
		log.Println(err)
		return false
	}

	if ok := p.node.sendProtoMessage(req, s); !ok {
		return false
	}

	return true
}
