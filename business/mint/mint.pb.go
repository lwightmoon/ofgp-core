// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/ofgp/ofgp-core/business/mint/mint.proto

package mint

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import proto1 "github.com/ofgp/ofgp-core/proto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type MintRequire struct {
	TokenFrom            uint32   `protobuf:"varint,1,opt,name=token_from,json=tokenFrom,proto3" json:"token_from,omitempty"`
	TokenTo              uint32   `protobuf:"varint,2,opt,name=token_to,json=tokenTo,proto3" json:"token_to,omitempty"`
	Receiver             []byte   `protobuf:"bytes,3,opt,name=receiver,proto3" json:"receiver,omitempty"`
	OpType               uint32   `protobuf:"varint,4,opt,name=op_type,json=opType,proto3" json:"op_type,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MintRequire) Reset()         { *m = MintRequire{} }
func (m *MintRequire) String() string { return proto.CompactTextString(m) }
func (*MintRequire) ProtoMessage()    {}
func (*MintRequire) Descriptor() ([]byte, []int) {
	return fileDescriptor_mint_08c8a1b393b661ea, []int{0}
}
func (m *MintRequire) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MintRequire.Unmarshal(m, b)
}
func (m *MintRequire) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MintRequire.Marshal(b, m, deterministic)
}
func (dst *MintRequire) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MintRequire.Merge(dst, src)
}
func (m *MintRequire) XXX_Size() int {
	return xxx_messageInfo_MintRequire.Size(m)
}
func (m *MintRequire) XXX_DiscardUnknown() {
	xxx_messageInfo_MintRequire.DiscardUnknown(m)
}

var xxx_messageInfo_MintRequire proto.InternalMessageInfo

func (m *MintRequire) GetTokenFrom() uint32 {
	if m != nil {
		return m.TokenFrom
	}
	return 0
}

func (m *MintRequire) GetTokenTo() uint32 {
	if m != nil {
		return m.TokenTo
	}
	return 0
}

func (m *MintRequire) GetReceiver() []byte {
	if m != nil {
		return m.Receiver
	}
	return nil
}

func (m *MintRequire) GetOpType() uint32 {
	if m != nil {
		return m.OpType
	}
	return 0
}

type MintInfo struct {
	Event                *proto1.WatchedEvent `protobuf:"bytes,1,opt,name=event,proto3" json:"event,omitempty"`
	Req                  *MintRequire         `protobuf:"bytes,2,opt,name=req,proto3" json:"req,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *MintInfo) Reset()         { *m = MintInfo{} }
func (m *MintInfo) String() string { return proto.CompactTextString(m) }
func (*MintInfo) ProtoMessage()    {}
func (*MintInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_mint_08c8a1b393b661ea, []int{1}
}
func (m *MintInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MintInfo.Unmarshal(m, b)
}
func (m *MintInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MintInfo.Marshal(b, m, deterministic)
}
func (dst *MintInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MintInfo.Merge(dst, src)
}
func (m *MintInfo) XXX_Size() int {
	return xxx_messageInfo_MintInfo.Size(m)
}
func (m *MintInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_MintInfo.DiscardUnknown(m)
}

var xxx_messageInfo_MintInfo proto.InternalMessageInfo

func (m *MintInfo) GetEvent() *proto1.WatchedEvent {
	if m != nil {
		return m.Event
	}
	return nil
}

func (m *MintInfo) GetReq() *MintRequire {
	if m != nil {
		return m.Req
	}
	return nil
}

func init() {
	proto.RegisterType((*MintRequire)(nil), "mint.mintRequire")
	proto.RegisterType((*MintInfo)(nil), "mint.mintInfo")
}

func init() {
	proto.RegisterFile("github.com/ofgp/ofgp-core/business/mint/mint.proto", fileDescriptor_mint_08c8a1b393b661ea)
}

var fileDescriptor_mint_08c8a1b393b661ea = []byte{
	// 245 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x90, 0x51, 0x4b, 0xc3, 0x30,
	0x10, 0xc7, 0xa9, 0x9b, 0x5b, 0xbd, 0xea, 0x83, 0xf1, 0xc1, 0x3a, 0x10, 0xc6, 0x7c, 0x99, 0x88,
	0x2d, 0xd4, 0xcf, 0xa0, 0xe0, 0x6b, 0x18, 0x08, 0xbe, 0x8c, 0xb5, 0x5e, 0xb7, 0x20, 0xc9, 0x65,
	0xd7, 0x74, 0x30, 0xf0, 0xc3, 0x4b, 0xae, 0x22, 0xbe, 0xf8, 0x72, 0xe4, 0xf2, 0xbb, 0x1f, 0xf7,
	0x4f, 0xa0, 0xda, 0x9a, 0xb0, 0xeb, 0xeb, 0xa2, 0x21, 0x5b, 0x52, 0xbb, 0xf5, 0x52, 0x1e, 0x1b,
	0x62, 0x2c, 0xeb, 0xbe, 0x33, 0x0e, 0xbb, 0xae, 0xb4, 0xc6, 0x05, 0x29, 0x85, 0x67, 0x0a, 0xa4,
	0xc6, 0xf1, 0x3c, 0x7b, 0xf8, 0xdf, 0x94, 0xb1, 0xb2, 0xe6, 0x4d, 0xfb, 0xa3, 0x2c, 0xbe, 0x20,
	0x8b, 0x92, 0xc6, 0x7d, 0x6f, 0x18, 0xd5, 0x2d, 0x40, 0xa0, 0x4f, 0x74, 0xeb, 0x96, 0xc9, 0xe6,
	0xc9, 0x3c, 0x59, 0x5e, 0xe8, 0x33, 0xb9, 0x79, 0x61, 0xb2, 0xea, 0x06, 0xd2, 0x01, 0x07, 0xca,
	0x4f, 0x04, 0x4e, 0xa5, 0x5f, 0x91, 0x9a, 0x41, 0xca, 0xd8, 0xa0, 0x39, 0x20, 0xe7, 0xa3, 0x79,
	0xb2, 0x3c, 0xd7, 0xbf, 0xbd, 0xba, 0x86, 0x29, 0xf9, 0x75, 0x38, 0x7a, 0xcc, 0xc7, 0x62, 0x4d,
	0xc8, 0xaf, 0x8e, 0x1e, 0x17, 0xef, 0x90, 0xc6, 0xed, 0xaf, 0xae, 0x25, 0x75, 0x0f, 0xa7, 0x78,
	0x40, 0x17, 0x64, 0x6b, 0x56, 0x5d, 0x0d, 0x01, 0x8b, 0xb7, 0x4d, 0x68, 0x76, 0xf8, 0xf1, 0x1c,
	0x91, 0x1e, 0x26, 0xd4, 0x1d, 0x8c, 0x18, 0xf7, 0x92, 0x20, 0xab, 0x2e, 0x0b, 0xf9, 0x81, 0x3f,
	0xaf, 0xd0, 0x91, 0xd6, 0x13, 0xf1, 0x9f, 0xbe, 0x03, 0x00, 0x00, 0xff, 0xff, 0x00, 0x18, 0x2b,
	0x20, 0x49, 0x01, 0x00, 0x00,
}
