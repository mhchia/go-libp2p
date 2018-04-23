package main

import (
	"bufio"
	"context"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	pstore "gx/ipfs/QmeZVQzUrXqaszo24DAoHfGzcmCptN9JyngLkGAiEfk2x7/go-libp2p-peerstore"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// pattern: /protocol-name/request-or-response-message/version
const notifyShardsRequest = "/notifyShards/request/0.0.1"
const notifyShardsResponse = "/notifyShards/response/0.0.1"

// NotifyShardsProtocol type
type NotifyShardsProtocol struct {
	node     *Node                                 // local host
	requests map[string]*pbmsg.NotifyShardsRequest // used to access request data from response handlers
	done     chan bool                             // only for demo purposes to stop main from terminating
}

func NewNotifyShardsProtocol(node *Node) *NotifyShardsProtocol {
	p := &NotifyShardsProtocol{
		node:     node,
		requests: make(map[string]*pbmsg.NotifyShardsRequest),
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

	log.Printf("%s: Received notifyShards request from %s. shardIDs: %v", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.ShardIDs)

}

func (p *NotifyShardsProtocol) NotifyShards(peerAddr string, shardIDs []int64) bool {
	peerid, targetAddr := parseAddr(peerAddr)
	log.Printf("%s: Sending notifyShards to: %s....", p.node.ID(), peerid)
	p.node.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)
	// create message data
	req := &pbmsg.NotifyShardsRequest{
		ShardIDs: shardIDs,
	}

	s, err := p.node.NewStream(context.Background(), peerid, notifyShardsRequest)
	if err != nil {
		log.Println(err)
		return false
	}

	if ok := p.node.sendProtoMessage(req, s); !ok {
		return false
	}

	// store ref request so response handler has access to it
	// p.requests[req.MessageData.Id] = req
	// log.Printf("%s: Ping to: %s was sent. Message Id: %s, Message: %s", p.node.ID(), host.ID(), req.MessageData.Id, req.Message)
	return true

}
