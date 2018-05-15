package main

import (
	"bufio"
	"context"
	"fmt"
	"log"

	"github.com/golang/protobuf/proto"

	"github.com/ethereum/go-ethereum/crypto"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"

	protocol "github.com/libp2p/go-libp2p-protocol"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// pattern: /protocol-name/request-or-response-message/version
const sendCollationRequestFmt = "/sendCollation%d/request/0.0.1"

const hashLength = 32

// ShardProtocol type
type ShardProtocol struct {
	node    *Node       // local host
	shardID ShardIDType // TODO: should be changed to `listeningShardIDs`
	// requests map[string]*pbmsg.SendCollationRequest // used to access request data from response handlers
	receivedCollations map[string]*pbmsg.SendCollationRequest
	done               chan bool // only for demo purposes to stop main from terminating
}

func getSendCollationRequestProtocolID(shardID ShardIDType) protocol.ID {
	return protocol.ID(fmt.Sprintf(sendCollationRequestFmt, shardID))
}

func NewShardProtocol(node *Node, shardID ShardIDType) *ShardProtocol {
	p := &ShardProtocol{
		node:               node,
		shardID:            shardID,
		receivedCollations: make(map[string]*pbmsg.SendCollationRequest),
		done:               make(chan bool),
	}
	node.SetStreamHandler(getSendCollationRequestProtocolID(shardID), p.receiveCollationRequest)
	return p
}

func Hash(msg *pbmsg.SendCollationRequest) string {
	dataInBytes, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("Error occurs when hashing %v", msg)
	}
	return string(crypto.Keccak256(dataInBytes))
}

// remote peer requests handler
func (p *ShardProtocol) receiveCollationRequest(s inet.Stream) {
	// reject if the sender is not a peer
	// TODO: confirm it works
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
	// TODO: temporarily comment out, to avoid the excessive memory usage
	// p.receivedCollations[Hash(data)] = data
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

func (p *ShardProtocol) sendCollation(peerID peer.ID, number int64, blobs string) bool {
	// create message data
	req := &pbmsg.SendCollationRequest{
		ShardID: p.shardID,
		Number:  number,
		Blobs:   blobs,
	}

	return p.sendCollationMessage(peerID, req)
}

func (p *ShardProtocol) sendCollationMessage(peerID peer.ID, req *pbmsg.SendCollationRequest) bool {
	log.Printf("%s: Sending collation to: %s....", p.node.ID(), peerID)

	s, err := p.node.NewStream(
		context.Background(),
		peerID,
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
