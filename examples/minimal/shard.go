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
		done:    make(chan bool),
	}
	node.SetStreamHandler(getSendCollationRequestProtocolID(shardID), p.sendCollationRequest)
	return p
}

// remote peer requests handler
func (p *ShardProtocol) sendCollationRequest(s inet.Stream) {
	// reject if the sender is not a peer
	// TODO: confirm it works
	if !p.node.IsPeer(s.Conn().LocalPeer()) {
		log.Printf(
			"%s: rejected sendCollationRequest from non-peer",
			s.Conn().LocalPeer(),
		)
		p.done <- false
		return
	}
	data := &pbmsg.SendCollationRequest{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		log.Println(err)
		p.done <- false
		return
	}
	// reject if the node isn't listening to the shard
	if !p.node.IsShardListened(data.ShardID) {
		log.Printf(
			"%s: Rejected sendCollationRequest %v of not listening shard %v",
			s.Conn().LocalPeer(),
			data,
			data.ShardID,
		)
		p.done <- false
		return
	}
	log.Printf(
		"%s: Received sendCollationRequest from %s. Message: shardID=%v, number=%v, blobs=%v",
		s.Conn().LocalPeer(),
		s.Conn().RemotePeer(),
		data.ShardID,
		data.Number,
		data.Blobs,
	)
	p.done <- true
}

func (p *ShardProtocol) sendCollation(peerAddr string, number int64, blobs string) bool {
	peerid, _ := parseAddr(peerAddr)
	log.Printf("%s: Sending collation to: %s....", p.node.ID(), peerid)
	// create message data
	req := &pbmsg.SendCollationRequest{
		ShardID: p.shardID,
		Number:  number,
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

	return true

}
