package main

import (
	"bufio"
	"context"
	"fmt"
	"log"

	inet "gx/ipfs/QmQm7WmgYCa4RSz76tKEYpRjApjnRw8ZTUVQC15b8JM4a2/go-libp2p-net"
	pstore "gx/ipfs/QmeZVQzUrXqaszo24DAoHfGzcmCptN9JyngLkGAiEfk2x7/go-libp2p-peerstore"

	pbmsg "github.com/libp2p/go-libp2p/examples/minimal/pb"

	protobufCodec "github.com/multiformats/go-multicodec/protobuf"
)

// pattern: /protocol-name/request-or-response-message/version
const addPeerRequest = "/addPeer/request/0.0.1"
const addPeerResponse = "/addPeer/response/0.0.1"

// AddPeerProtocol type
type AddPeerProtocol struct {
	node     *Node                            // local host
	requests map[string]*pbmsg.AddPeerRequest // used to access request data from response handlers
}

func NewAddPeerProtocol(node *Node) *AddPeerProtocol {
	p := &AddPeerProtocol{
		node:     node,
		requests: make(map[string]*pbmsg.AddPeerRequest),
	}
	node.SetStreamHandler(addPeerRequest, p.onRequest)
	node.SetStreamHandler(addPeerResponse, p.onResponse)
	return p
}

// remote peer requests handler
func (p *AddPeerProtocol) onRequest(s inet.Stream) {

	// get request data
	data := &pbmsg.AddPeerRequest{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("%s: Received addPeer request from %s. Message: %s", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Message)

	// generate response message
	log.Printf("%s: Sending addPeer response to %s", s.Conn().LocalPeer(), s.Conn().RemotePeer())

	resp := &pbmsg.AddPeerResponse{
		Success: true,
	}

	// send the response
	s, respErr := p.node.NewStream(context.Background(), s.Conn().RemotePeer(), addPeerResponse)
	if respErr != nil {
		log.Println(respErr)
		return
	}

	if ok := p.node.sendProtoMessage(resp, s); ok {
		log.Printf("%s: AddPeer response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
}

// remote addPeer response handler
func (p *AddPeerProtocol) onResponse(s inet.Stream) {
	data := &pbmsg.AddPeerResponse{}
	decoder := protobufCodec.Multicodec(nil).Decoder(bufio.NewReader(s))
	err := decoder.Decode(data)
	if err != nil {
		return
	}

	log.Printf(
		"%s: Received addPeer response from %s, result=%v",
		s.Conn().LocalPeer(),
		s.Conn().RemotePeer(),
		data.Success,
	)
	// locate request data and remove it if found
	// _, ok := p.requests[data.MessageData.Id]
	// if ok {
	// 	// remove request from map as we have processed it here
	// 	delete(p.requests, data.MessageData.Id)
	// } else {
	// 	log.Println("Failed to locate request data boject for response")
	// 	return
	// }
}

func (p *AddPeerProtocol) AddPeer(peerAddr string) bool {
	peerid, targetAddr := parseAddr(peerAddr)
	log.Printf("%s: Sending addPeer to: %s....", p.node.ID(), peerid)
	p.node.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)
	// create message data
	req := &pbmsg.AddPeerRequest{
		Message: fmt.Sprintf("AddPeer from %s", p.node.ID()),
	}

	s, err := p.node.NewStream(context.Background(), peerid, addPeerRequest)
	if err != nil {
		log.Println(err)
		return false
	}

	if ok := p.node.sendProtoMessage(req, s); !ok {
		return false
	}

	// store ref request so response handler has access to it
	// p.requests[req.MessageData.Id] = req
	// log.Printf("%s: AddPeer to: %s was sent. Message Id: %s, Message: %s", p.node.ID(), host.ID(), req.MessageData.Id, req.Message)
	return true

}
