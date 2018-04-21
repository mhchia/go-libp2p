// Code generated by protoc-gen-go. DO NOT EDIT.
// source: minimal.proto

/*
Package protocols_p2p is a generated protocol buffer package.

It is generated from these files:
	minimal.proto

It has these top-level messages:
	AddPeerRequest
	AddPeerResponse
*/
package protocols_p2p

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// a protocol define a set of reuqest and responses
type AddPeerRequest struct {
	// method specific data
	Message string `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
}

func (m *AddPeerRequest) Reset()                    { *m = AddPeerRequest{} }
func (m *AddPeerRequest) String() string            { return proto.CompactTextString(m) }
func (*AddPeerRequest) ProtoMessage()               {}
func (*AddPeerRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *AddPeerRequest) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

type AddPeerResponse struct {
	// response specific data
	Message string `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
}

func (m *AddPeerResponse) Reset()                    { *m = AddPeerResponse{} }
func (m *AddPeerResponse) String() string            { return proto.CompactTextString(m) }
func (*AddPeerResponse) ProtoMessage()               {}
func (*AddPeerResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *AddPeerResponse) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func init() {
	proto.RegisterType((*AddPeerRequest)(nil), "protocols.p2p.AddPeerRequest")
	proto.RegisterType((*AddPeerResponse)(nil), "protocols.p2p.AddPeerResponse")
}

func init() { proto.RegisterFile("minimal.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 108 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcd, 0xcd, 0xcc, 0xcb,
	0xcc, 0x4d, 0xcc, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x05, 0x53, 0xc9, 0xf9, 0x39,
	0xc5, 0x7a, 0x05, 0x46, 0x05, 0x4a, 0x5a, 0x5c, 0x7c, 0x8e, 0x29, 0x29, 0x01, 0xa9, 0xa9, 0x45,
	0x41, 0xa9, 0x85, 0xa5, 0xa9, 0xc5, 0x25, 0x42, 0x12, 0x5c, 0xec, 0xb9, 0xa9, 0xc5, 0xc5, 0x89,
	0xe9, 0xa9, 0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0x9c, 0x41, 0x30, 0xae, 0x92, 0x36, 0x17, 0x3f, 0x5c,
	0x6d, 0x71, 0x41, 0x7e, 0x5e, 0x71, 0x2a, 0x6e, 0xc5, 0x49, 0x6c, 0x60, 0x7b, 0x8c, 0x01, 0x01,
	0x00, 0x00, 0xff, 0xff, 0x16, 0x47, 0xfe, 0x39, 0x7f, 0x00, 0x00, 0x00,
}
