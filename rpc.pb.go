// Code generated by protoc-gen-go.
// source: rpc.proto
// DO NOT EDIT!

/*
Package hexalog is a generated protocol buffer package.

It is generated from these files:
	rpc.proto

It has these top-level messages:
*/
package hexalog

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import hexatype "github.com/hexablock/hexatype"

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

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for HexalogRPC service

type HexalogRPCClient interface {
	ProposeRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error)
	CommitRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error)
	GetRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error)
	TransferKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_TransferKeylogRPCClient, error)
	FetchKeylogRPC(ctx context.Context, opts ...grpc.CallOption) (HexalogRPC_FetchKeylogRPCClient, error)
}

type hexalogRPCClient struct {
	cc *grpc.ClientConn
}

func NewHexalogRPCClient(cc *grpc.ClientConn) HexalogRPCClient {
	return &hexalogRPCClient{cc}
}

func (c *hexalogRPCClient) ProposeRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error) {
	out := new(hexatype.ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/ProposeRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) CommitRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error) {
	out := new(hexatype.ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/CommitRPC", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hexalogRPCClient) GetRPC(ctx context.Context, in *hexatype.ReqResp, opts ...grpc.CallOption) (*hexatype.ReqResp, error) {
	out := new(hexatype.ReqResp)
	err := grpc.Invoke(ctx, "/hexalog.HexalogRPC/GetRPC", in, out, c.cc, opts...)
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
	Send(*hexatype.ReqResp) error
	Recv() (*hexatype.ReqResp, error)
	grpc.ClientStream
}

type hexalogRPCTransferKeylogRPCClient struct {
	grpc.ClientStream
}

func (x *hexalogRPCTransferKeylogRPCClient) Send(m *hexatype.ReqResp) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hexalogRPCTransferKeylogRPCClient) Recv() (*hexatype.ReqResp, error) {
	m := new(hexatype.ReqResp)
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
	Send(*hexatype.ReqResp) error
	Recv() (*hexatype.ReqResp, error)
	grpc.ClientStream
}

type hexalogRPCFetchKeylogRPCClient struct {
	grpc.ClientStream
}

func (x *hexalogRPCFetchKeylogRPCClient) Send(m *hexatype.ReqResp) error {
	return x.ClientStream.SendMsg(m)
}

func (x *hexalogRPCFetchKeylogRPCClient) Recv() (*hexatype.ReqResp, error) {
	m := new(hexatype.ReqResp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for HexalogRPC service

type HexalogRPCServer interface {
	ProposeRPC(context.Context, *hexatype.ReqResp) (*hexatype.ReqResp, error)
	CommitRPC(context.Context, *hexatype.ReqResp) (*hexatype.ReqResp, error)
	GetRPC(context.Context, *hexatype.ReqResp) (*hexatype.ReqResp, error)
	TransferKeylogRPC(HexalogRPC_TransferKeylogRPCServer) error
	FetchKeylogRPC(HexalogRPC_FetchKeylogRPCServer) error
}

func RegisterHexalogRPCServer(s *grpc.Server, srv HexalogRPCServer) {
	s.RegisterService(&_HexalogRPC_serviceDesc, srv)
}

func _HexalogRPC_ProposeRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(hexatype.ReqResp)
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
		return srv.(HexalogRPCServer).ProposeRPC(ctx, req.(*hexatype.ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_CommitRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(hexatype.ReqResp)
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
		return srv.(HexalogRPCServer).CommitRPC(ctx, req.(*hexatype.ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_GetRPC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(hexatype.ReqResp)
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
		return srv.(HexalogRPCServer).GetRPC(ctx, req.(*hexatype.ReqResp))
	}
	return interceptor(ctx, in, info, handler)
}

func _HexalogRPC_TransferKeylogRPC_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HexalogRPCServer).TransferKeylogRPC(&hexalogRPCTransferKeylogRPCServer{stream})
}

type HexalogRPC_TransferKeylogRPCServer interface {
	Send(*hexatype.ReqResp) error
	Recv() (*hexatype.ReqResp, error)
	grpc.ServerStream
}

type hexalogRPCTransferKeylogRPCServer struct {
	grpc.ServerStream
}

func (x *hexalogRPCTransferKeylogRPCServer) Send(m *hexatype.ReqResp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hexalogRPCTransferKeylogRPCServer) Recv() (*hexatype.ReqResp, error) {
	m := new(hexatype.ReqResp)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _HexalogRPC_FetchKeylogRPC_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(HexalogRPCServer).FetchKeylogRPC(&hexalogRPCFetchKeylogRPCServer{stream})
}

type HexalogRPC_FetchKeylogRPCServer interface {
	Send(*hexatype.ReqResp) error
	Recv() (*hexatype.ReqResp, error)
	grpc.ServerStream
}

type hexalogRPCFetchKeylogRPCServer struct {
	grpc.ServerStream
}

func (x *hexalogRPCFetchKeylogRPCServer) Send(m *hexatype.ReqResp) error {
	return x.ServerStream.SendMsg(m)
}

func (x *hexalogRPCFetchKeylogRPCServer) Recv() (*hexatype.ReqResp, error) {
	m := new(hexatype.ReqResp)
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
			MethodName: "GetRPC",
			Handler:    _HexalogRPC_GetRPC_Handler,
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
	// 175 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2c, 0x2a, 0x48, 0xd6,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0xcf, 0x48, 0xad, 0x48, 0xcc, 0xc9, 0x4f, 0x97, 0xd2,
	0x4c, 0xcf, 0x2c, 0xc9, 0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x07, 0x89, 0x25, 0xe5, 0xe4,
	0x27, 0x67, 0x83, 0x59, 0x25, 0x95, 0x05, 0xa9, 0xfa, 0x20, 0xa2, 0x18, 0xa2, 0xc7, 0x68, 0x05,
	0x13, 0x17, 0x97, 0x07, 0x44, 0x5b, 0x50, 0x80, 0xb3, 0x90, 0x09, 0x17, 0x57, 0x40, 0x51, 0x7e,
	0x41, 0x7e, 0x71, 0x2a, 0x88, 0x27, 0xa8, 0x07, 0xd3, 0xa3, 0x17, 0x94, 0x5a, 0x18, 0x94, 0x5a,
	0x5c, 0x20, 0x85, 0x29, 0xa4, 0xc4, 0x20, 0x64, 0xcc, 0xc5, 0xe9, 0x9c, 0x9f, 0x9b, 0x9b, 0x59,
	0x42, 0x8a, 0x26, 0x03, 0x2e, 0x36, 0xf7, 0x54, 0x92, 0x74, 0xd8, 0x73, 0x09, 0x86, 0x14, 0x25,
	0xe6, 0x15, 0xa7, 0xa5, 0x16, 0x79, 0xa7, 0x56, 0x42, 0x5d, 0x4c, 0xa4, 0x66, 0x0d, 0x46, 0x03,
	0x46, 0x21, 0x1b, 0x2e, 0x3e, 0xb7, 0xd4, 0x92, 0xe4, 0x0c, 0xb2, 0x74, 0x27, 0xb1, 0x81, 0x43,
	0xcc, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x47, 0x88, 0xe7, 0xec, 0x72, 0x01, 0x00, 0x00,
}
