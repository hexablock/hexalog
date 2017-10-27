// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

/*
Package hexalog is a generated protocol buffer package.

It is generated from these files:
	rpc.proto

It has these top-level messages:
	Entry
	UnsafeKeylogIndex
	ReqResp
	RequestOptions
	Participant
*/
package hexalog

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// Hexalog entry
type Entry struct {
	Key       []byte `protobuf:"bytes,1,opt,name=Key,proto3" json:"Key,omitempty"`
	Previous  []byte `protobuf:"bytes,2,opt,name=Previous,proto3" json:"Previous,omitempty"`
	Height    uint32 `protobuf:"varint,3,opt,name=Height" json:"Height,omitempty"`
	Timestamp uint64 `protobuf:"varint,4,opt,name=Timestamp" json:"Timestamp,omitempty"`
	LTime     uint64 `protobuf:"varint,5,opt,name=LTime" json:"LTime,omitempty"`
	Data      []byte `protobuf:"bytes,6,opt,name=Data,proto3" json:"Data,omitempty"`
}

func (m *Entry) Reset()                    { *m = Entry{} }
func (m *Entry) String() string            { return proto.CompactTextString(m) }
func (*Entry) ProtoMessage()               {}
func (*Entry) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Entry) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *Entry) GetPrevious() []byte {
	if m != nil {
		return m.Previous
	}
	return nil
}

func (m *Entry) GetHeight() uint32 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *Entry) GetTimestamp() uint64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

func (m *Entry) GetLTime() uint64 {
	if m != nil {
		return m.LTime
	}
	return 0
}

func (m *Entry) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

// UnsafeKeylogIndex is an in-memory keylog index. This is the base class for all
// implementations of KeylogIndex
type UnsafeKeylogIndex struct {
	// Key for the index
	Key []byte `protobuf:"bytes,1,opt,name=Key,proto3" json:"Key,omitempty"`
	// Current height of the keylog
	Height uint32 `protobuf:"varint,2,opt,name=Height" json:"Height,omitempty"`
	// Used to mark an incomplete log
	Marker []byte `protobuf:"bytes,3,opt,name=Marker,proto3" json:"Marker,omitempty"`
	// Entry ids
	Entries [][]byte `protobuf:"bytes,4,rep,name=Entries,proto3" json:"Entries,omitempty"`
}

func (m *UnsafeKeylogIndex) Reset()                    { *m = UnsafeKeylogIndex{} }
func (m *UnsafeKeylogIndex) String() string            { return proto.CompactTextString(m) }
func (*UnsafeKeylogIndex) ProtoMessage()               {}
func (*UnsafeKeylogIndex) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *UnsafeKeylogIndex) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *UnsafeKeylogIndex) GetHeight() uint32 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *UnsafeKeylogIndex) GetMarker() []byte {
	if m != nil {
		return m.Marker
	}
	return nil
}

func (m *UnsafeKeylogIndex) GetEntries() [][]byte {
	if m != nil {
		return m.Entries
	}
	return nil
}

// Request and response shared structure for hexalog
type ReqResp struct {
	// ID is based on the request/response
	ID      []byte          `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Entry   *Entry          `protobuf:"bytes,2,opt,name=Entry" json:"Entry,omitempty"`
	Options *RequestOptions `protobuf:"bytes,3,opt,name=Options" json:"Options,omitempty"`
	// Response fields
	BallotTime int64 `protobuf:"varint,4,opt,name=BallotTime" json:"BallotTime,omitempty"`
	ApplyTime  int64 `protobuf:"varint,5,opt,name=ApplyTime" json:"ApplyTime,omitempty"`
}

func (m *ReqResp) Reset()                    { *m = ReqResp{} }
func (m *ReqResp) String() string            { return proto.CompactTextString(m) }
func (*ReqResp) ProtoMessage()               {}
func (*ReqResp) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *ReqResp) GetID() []byte {
	if m != nil {
		return m.ID
	}
	return nil
}

func (m *ReqResp) GetEntry() *Entry {
	if m != nil {
		return m.Entry
	}
	return nil
}

func (m *ReqResp) GetOptions() *RequestOptions {
	if m != nil {
		return m.Options
	}
	return nil
}

func (m *ReqResp) GetBallotTime() int64 {
	if m != nil {
		return m.BallotTime
	}
	return 0
}

func (m *ReqResp) GetApplyTime() int64 {
	if m != nil {
		return m.ApplyTime
	}
	return 0
}

// Hexalog request options
type RequestOptions struct {
	// Index of the source in the PeerSet.  This is set internally by the log
	SourceIndex int32 `protobuf:"varint,1,opt,name=SourceIndex" json:"SourceIndex,omitempty"`
	// Set of peers for the request.
	PeerSet []*Participant `protobuf:"bytes,2,rep,name=PeerSet" json:"PeerSet,omitempty"`
	// Wait on ballot before returning
	WaitBallot bool `protobuf:"varint,5,opt,name=WaitBallot" json:"WaitBallot,omitempty"`
	// Wait for fsm to apply entry after ballot is closed. This should take
	// effect only if WaitBallot is also true
	WaitApply bool `protobuf:"varint,6,opt,name=WaitApply" json:"WaitApply,omitempty"`
	// Apply timeout in ms.  This only takes effect if WaitApply is also set
	WaitApplyTimeout int32 `protobuf:"varint,7,opt,name=WaitApplyTimeout" json:"WaitApplyTimeout,omitempty"`
	// Lamport time
	LTime uint64 `protobuf:"varint,8,opt,name=LTime" json:"LTime,omitempty"`
}

func (m *RequestOptions) Reset()                    { *m = RequestOptions{} }
func (m *RequestOptions) String() string            { return proto.CompactTextString(m) }
func (*RequestOptions) ProtoMessage()               {}
func (*RequestOptions) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *RequestOptions) GetSourceIndex() int32 {
	if m != nil {
		return m.SourceIndex
	}
	return 0
}

func (m *RequestOptions) GetPeerSet() []*Participant {
	if m != nil {
		return m.PeerSet
	}
	return nil
}

func (m *RequestOptions) GetWaitBallot() bool {
	if m != nil {
		return m.WaitBallot
	}
	return false
}

func (m *RequestOptions) GetWaitApply() bool {
	if m != nil {
		return m.WaitApply
	}
	return false
}

func (m *RequestOptions) GetWaitApplyTimeout() int32 {
	if m != nil {
		return m.WaitApplyTimeout
	}
	return 0
}

func (m *RequestOptions) GetLTime() uint64 {
	if m != nil {
		return m.LTime
	}
	return 0
}

type Participant struct {
	ID []byte `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	// Host:port
	Host string `protobuf:"bytes,2,opt,name=Host" json:"Host,omitempty"`
	// Priority among locations in a set
	Priority int32 `protobuf:"varint,3,opt,name=Priority" json:"Priority,omitempty"`
	// Index within location group
	Index int32 `protobuf:"varint,4,opt,name=Index" json:"Index,omitempty"`
}

func (m *Participant) Reset()                    { *m = Participant{} }
func (m *Participant) String() string            { return proto.CompactTextString(m) }
func (*Participant) ProtoMessage()               {}
func (*Participant) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *Participant) GetID() []byte {
	if m != nil {
		return m.ID
	}
	return nil
}

func (m *Participant) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *Participant) GetPriority() int32 {
	if m != nil {
		return m.Priority
	}
	return 0
}

func (m *Participant) GetIndex() int32 {
	if m != nil {
		return m.Index
	}
	return 0
}

func init() {
	proto.RegisterType((*Entry)(nil), "hexalog.Entry")
	proto.RegisterType((*UnsafeKeylogIndex)(nil), "hexalog.UnsafeKeylogIndex")
	proto.RegisterType((*ReqResp)(nil), "hexalog.ReqResp")
	proto.RegisterType((*RequestOptions)(nil), "hexalog.RequestOptions")
	proto.RegisterType((*Participant)(nil), "hexalog.Participant")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for HexalogRPC service

type HexalogRPCClient interface {
	ProposeRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error)
	CommitRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error)
	NewRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error)
	GetRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error)
	LastRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error)
	TransferKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_TransferKeylogRPCClient, error)
	FetchKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_FetchKeylogRPCClient, error)
}

type hexalogRPCClient struct {
	cc *grpc.ClientConn
}

func NewHexalogRPCClient(cc *grpc.ClientConn) HexalogRPCClient {
	return &hexalogRPCClient{cc}
}

func (c *hexalogRPCClient) ProposeRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error) {
	out := new(ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/ProposeRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) CommitRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error) {
	out := new(ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/CommitRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) NewRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error) {
	out := new(ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/NewRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) GetRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error) {
	out := new(ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/GetRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) LastRPC(ctx context.Context, in *ReqResp, opts ...grpc.CallOption) (*ReqResp, error) {
	out := new(ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/LastRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) TransferKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_TransferKeylogRPCClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_HexalogRPC_serviceDesc.Streams[0], c.cc, "/hexalog.HexalogRPC/TransferKeylogRPC", opts...)
	if err != nil {
		return nil, err
	}
	x := &hexalogRPCTransferKeylogRPCClient{stream}
	return x, nil
}

type HexalogRPC_TransferKeylogRPCClient interface {
	Send(*ReqResp) error
	Recv() (*ReqResp, error)
	grpc.ClientStream
}

type hexalogRPCTransferKeylogRPCClient struct {
	grpc.ClientStream
}

func (x *hexalogRPCTransferKeylogRPCClient) Send(m *ReqResp) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hexalogRPCTransferKeylogRPCClient) Recv() (*ReqResp, error) {
	m := new(ReqResp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *hexalogRPCClient) FetchKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_FetchKeylogRPCClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_HexalogRPC_serviceDesc.Streams[1], c.cc, "/hexalog.HexalogRPC/FetchKeylogRPC", opts...)
	if err != nil {
		return nil, err
	}
	x := &hexalogRPCFetchKeylogRPCClient{stream}
	return x, nil
}

type HexalogRPC_FetchKeylogRPCClient interface {
	Send(*ReqResp) error
	Recv() (*ReqResp, error)
	grpc.ClientStream
}

type hexalogRPCFetchKeylogRPCClient struct {
	grpc.ClientStream
}

func (x *hexalogRPCFetchKeylogRPCClient) Send(m *ReqResp) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hexalogRPCFetchKeylogRPCClient) Recv() (*ReqResp, error) {
	m := new(ReqResp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for HexalogRPC service

type HexalogRPCServer interface {
	ProposeRPC(context.Context, *ReqResp) (*ReqResp, error)
	CommitRPC(context.Context, *ReqResp) (*ReqResp, error)
	NewRPC(context.Context, *ReqResp) (*ReqResp, error)
	GetRPC(context.Context, *ReqResp) (*ReqResp, error)
	LastRPC(context.Context, *ReqResp) (*ReqResp, error)
	TransferKeylogRPC(HexalogRPC_TransferKeylogRPCServer) error
	FetchKeylogRPC(HexalogRPC_FetchKeylogRPCServer) error
}

func RegisterHexalogRPCServer(s *grpc.Server, srv HexalogRPCServer) {
	s.RegisterService(&_HexalogRPC_serviceDesc, srv)
}

func _HexalogRPC_ProposeRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqResp)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HexalogRPCServer).ProposeRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hexalog.HexalogRPC/ProposeRPC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HexalogRPCServer).ProposeRPC(ctx, req.(*ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_CommitRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqResp)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HexalogRPCServer).CommitRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hexalog.HexalogRPC/CommitRPC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HexalogRPCServer).CommitRPC(ctx, req.(*ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_NewRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqResp)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HexalogRPCServer).NewRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hexalog.HexalogRPC/NewRPC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HexalogRPCServer).NewRPC(ctx, req.(*ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_GetRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqResp)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HexalogRPCServer).GetRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hexalog.HexalogRPC/GetRPC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HexalogRPCServer).GetRPC(ctx, req.(*ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_LastRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqResp)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HexalogRPCServer).LastRPC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hexalog.HexalogRPC/LastRPC",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HexalogRPCServer).LastRPC(ctx, req.(*ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_TransferKeylogRPC_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HexalogRPCServer).TransferKeylogRPC(&hexalogRPCTransferKeylogRPCServer{stream})
}

type HexalogRPC_TransferKeylogRPCServer interface {
	Send(*ReqResp) error
	Recv() (*ReqResp, error)
	grpc.ServerStream
}

type hexalogRPCTransferKeylogRPCServer struct {
	grpc.ServerStream
}

func (x *hexalogRPCTransferKeylogRPCServer) Send(m *ReqResp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hexalogRPCTransferKeylogRPCServer) Recv() (*ReqResp, error) {
	m := new(ReqResp)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _HexalogRPC_FetchKeylogRPC_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HexalogRPCServer).FetchKeylogRPC(&hexalogRPCFetchKeylogRPCServer{stream})
}

type HexalogRPC_FetchKeylogRPCServer interface {
	Send(*ReqResp) error
	Recv() (*ReqResp, error)
	grpc.ServerStream
}

type hexalogRPCFetchKeylogRPCServer struct {
	grpc.ServerStream
}

func (x *hexalogRPCFetchKeylogRPCServer) Send(m *ReqResp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hexalogRPCFetchKeylogRPCServer) Recv() (*ReqResp, error) {
	m := new(ReqResp)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _HexalogRPC_serviceDesc = grpc.ServiceDesc{
	ServiceName: "hexalog.HexalogRPC",
	HandlerType: (*HexalogRPCServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ProposeRPC",
			Handler:    _HexalogRPC_ProposeRPC_Handler,
		},
		{
			MethodName: "CommitRPC",
			Handler:    _HexalogRPC_CommitRPC_Handler,
		},
		{
			MethodName: "NewRPC",
			Handler:    _HexalogRPC_NewRPC_Handler,
		},
		{
			MethodName: "GetRPC",
			Handler:    _HexalogRPC_GetRPC_Handler,
		},
		{
			MethodName: "LastRPC",
			Handler:    _HexalogRPC_LastRPC_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "TransferKeylogRPC",
			Handler:       _HexalogRPC_TransferKeylogRPC_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "FetchKeylogRPC",
			Handler:       _HexalogRPC_FetchKeylogRPC_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "rpc.proto",
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 529 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x54, 0xc1, 0x6e, 0xd3, 0x40,
	0x10, 0xc5, 0xb1, 0x13, 0x27, 0x93, 0x10, 0xa5, 0xab, 0xaa, 0x58, 0x15, 0x42, 0x91, 0xc5, 0xc1,
	0xe2, 0x10, 0x68, 0xb8, 0x21, 0x71, 0x80, 0x06, 0x48, 0xd4, 0x02, 0xd1, 0xb6, 0x88, 0xf3, 0x12,
	0xa6, 0xc9, 0x0a, 0xc7, 0xeb, 0xee, 0x6e, 0xa0, 0xfe, 0x10, 0xfe, 0x83, 0x6f, 0xe2, 0x47, 0x40,
	0xbb, 0x76, 0x6c, 0x57, 0xe1, 0xd0, 0x70, 0xdb, 0xf7, 0x66, 0x66, 0xe7, 0xed, 0x9b, 0xb1, 0xa1,
	0x23, 0xd3, 0xc5, 0x28, 0x95, 0x42, 0x0b, 0xe2, 0xaf, 0xf0, 0x86, 0xc5, 0x62, 0x19, 0xfe, 0x74,
	0xa0, 0xf9, 0x26, 0xd1, 0x32, 0x23, 0x03, 0x70, 0xcf, 0x30, 0x0b, 0x9c, 0xa1, 0x13, 0xf5, 0xa8,
	0x39, 0x92, 0x63, 0x68, 0xcf, 0x25, 0x7e, 0xe7, 0x62, 0xa3, 0x82, 0x86, 0xa5, 0x4b, 0x4c, 0x8e,
	0xa0, 0x35, 0x45, 0xbe, 0x5c, 0xe9, 0xc0, 0x1d, 0x3a, 0xd1, 0x7d, 0x5a, 0x20, 0xf2, 0x10, 0x3a,
	0x97, 0x7c, 0x8d, 0x4a, 0xb3, 0x75, 0x1a, 0x78, 0x43, 0x27, 0xf2, 0x68, 0x45, 0x90, 0x43, 0x68,
	0x9e, 0x1b, 0x14, 0x34, 0x6d, 0x24, 0x07, 0x84, 0x80, 0x37, 0x61, 0x9a, 0x05, 0x2d, 0xdb, 0xc3,
	0x9e, 0x43, 0x01, 0x07, 0x9f, 0x12, 0xc5, 0xae, 0xf0, 0x0c, 0xb3, 0x58, 0x2c, 0x67, 0xc9, 0x57,
	0xbc, 0xf9, 0x87, 0xc4, 0x4a, 0x46, 0xe3, 0x96, 0x8c, 0x23, 0x68, 0xbd, 0x67, 0xf2, 0x1b, 0x4a,
	0x2b, 0xaf, 0x47, 0x0b, 0x44, 0x02, 0xf0, 0xcd, 0x6b, 0x39, 0xaa, 0xc0, 0x1b, 0xba, 0x51, 0x8f,
	0x6e, 0x61, 0xf8, 0xcb, 0x01, 0x9f, 0xe2, 0x35, 0x45, 0x95, 0x92, 0x3e, 0x34, 0x66, 0x93, 0xa2,
	0x4d, 0x63, 0x36, 0x21, 0x8f, 0x0b, 0x8f, 0x6c, 0x93, 0xee, 0xb8, 0x3f, 0x2a, 0xdc, 0x1b, 0x59,
	0x96, 0x16, 0x06, 0x9e, 0x80, 0xff, 0x31, 0xd5, 0x5c, 0x24, 0xca, 0x36, 0xed, 0x8e, 0x1f, 0x94,
	0x79, 0x14, 0xaf, 0x37, 0xa8, 0x74, 0x11, 0xa6, 0xdb, 0x3c, 0xf2, 0x08, 0xe0, 0x35, 0x8b, 0x63,
	0xa1, 0xad, 0x29, 0xc6, 0x2e, 0x97, 0xd6, 0x18, 0xe3, 0xe6, 0xab, 0x34, 0x8d, 0xb3, 0xd2, 0x33,
	0x97, 0x56, 0x44, 0xf8, 0xdb, 0x81, 0xfe, 0xed, 0x9b, 0xc9, 0x10, 0xba, 0x17, 0x62, 0x23, 0x17,
	0x68, 0x0d, 0xb3, 0x4f, 0x68, 0xd2, 0x3a, 0x45, 0x46, 0xe0, 0xcf, 0x11, 0xe5, 0x05, 0x1a, 0xcb,
	0xdc, 0xa8, 0x3b, 0x3e, 0x2c, 0x55, 0xce, 0x99, 0xd4, 0x7c, 0xc1, 0x53, 0x96, 0x68, 0xba, 0x4d,
	0x32, 0x12, 0x3f, 0x33, 0xae, 0x73, 0x51, 0x56, 0x43, 0x9b, 0xd6, 0x18, 0x23, 0xd1, 0x20, 0xab,
	0xca, 0x4e, 0xb0, 0x4d, 0x2b, 0x82, 0x3c, 0x81, 0x41, 0x09, 0x8c, 0x66, 0xb1, 0xd1, 0x81, 0x6f,
	0x45, 0xed, 0xf0, 0xd5, 0x72, 0xb4, 0x6b, 0xcb, 0x11, 0x2e, 0xa0, 0x5b, 0xd3, 0xb5, 0x33, 0x1a,
	0x02, 0xde, 0x54, 0xa8, 0x7c, 0xfc, 0x1d, 0x6a, 0xcf, 0xf9, 0xde, 0x72, 0x21, 0xb9, 0xce, 0xec,
	0x24, 0x9a, 0xb4, 0xc4, 0xa6, 0x49, 0x6e, 0x8d, 0x67, 0x03, 0x39, 0x18, 0xff, 0x69, 0x00, 0x4c,
	0x73, 0x17, 0xe8, 0xfc, 0x94, 0x8c, 0x01, 0xe6, 0x52, 0xa4, 0x42, 0xa1, 0x41, 0x83, 0xfa, 0x18,
	0xcd, 0x7e, 0x1c, 0xef, 0x30, 0xe1, 0x3d, 0x72, 0x02, 0x9d, 0x53, 0xb1, 0x5e, 0x73, 0x7d, 0xf7,
	0x92, 0x11, 0xb4, 0x3e, 0xe0, 0x8f, 0xbd, 0xf2, 0xdf, 0xe1, 0x1e, 0xf7, 0x3f, 0x05, 0xff, 0x9c,
	0xa9, 0x3d, 0x0a, 0x5e, 0xc2, 0xc1, 0xa5, 0x64, 0x89, 0xba, 0x42, 0x99, 0x7f, 0x76, 0x77, 0x2e,
	0x8d, 0x9c, 0x67, 0x0e, 0x79, 0x01, 0xfd, 0xb7, 0xa8, 0x17, 0xab, 0xff, 0xa8, 0xfd, 0xd2, 0xb2,
	0xff, 0xa5, 0xe7, 0x7f, 0x03, 0x00, 0x00, 0xff, 0xff, 0x3f, 0xa2, 0xdd, 0x7b, 0xa4, 0x04, 0x00,
	0x00,
}
