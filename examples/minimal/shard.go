package main

import (
	"bufio"
	"context"
	"fmt"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"

	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// pattern: /protocol-name/request-or-response-message/version
const sendCollationRequestFmt = "/sendCollation%d/request/0.0.1"

// ShardProtocol type
type ShardProtocol struct {
	node    *Node       // local host
	shardID ShardIDType // TODO: should be changed to `listeningShardIDs`
	// requests map[string]*pbmsg.SendCollationRequest // used to access request data from response handlers
	done chan bool // only for demo purposes to stop main from terminating
}

func getSendCollationRequestProtocolID(shardID ShardIDType) protocol.ID {
	return protocol.ID(fmt.Sprintf(sendCollationRequestFmt, shardID))
}

func NewShardProtocol(node *Node, shardID ShardIDType) *ShardProtocol {
	p := &ShardProtocol{
		node:    node,
		shardID: shardID,
	}
	node.SetStreamHandler(getSendCollationRequestProtocolID(shardID), p.sendCollationRequest)
	return p
}

// remote peer requests handler
func (p *ShardProtocol) sendCollationRequest(s inet.Stream) {
	// TODO: should reject if the node isn't listening to the shard
	data := &pbmsg.SendCollationRequest{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf(
		"%s: Received sendCollationRequest from %s. Message: shardID=%d, blobs=%s",
		s.Conn().LocalPeer(),
		s.Conn().RemotePeer(),
		data.ShardID,
		data.Blobs,
	)
}

func (p *ShardProtocol) sendCollation(peerAddr string, blobs string) bool {
	peerid, _ := parseAddr(peerAddr)
	log.Printf("%s: Sending collation to: %s....", p.node.ID(), peerid)
	// create message data
	req := &pbmsg.SendCollationRequest{
		ShardID: int64(p.shardID),
		Blobs:   blobs,
	}

	s, err := p.node.NewStream(
		context.Background(),
		peerid,
		getSendCollationRequestProtocolID(p.shardID),
	)
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
