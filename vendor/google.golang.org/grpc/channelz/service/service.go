/*
 *
 * Copyright 2018 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

//go:generate protoc -I ../grpc_channelz_v1 --go_out=plugins=grpc,paths=source_relative:../grpc_channelz_v1 ../grpc_channelz_v1/channelz.proto

// Package service provides an implementation for channelz service server.
package service

import (
	"net"

	"github.com/golang/protobuf/ptypes"
	wrpb "github.com/golang/protobuf/ptypes/wrappers"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz"
	pb "google.golang.org/grpc/channelz/grpc_channelz_v1"
	"google.golang.org/grpc/connectivity"
)

// RegisterChannelzServiceToServer registers the channelz service to the given server.
func RegisterChannelzServiceToServer(s *grpc.Server) {
	pb.RegisterChannelzServer(s, &serverImpl{})
}

func newCZServer() pb.ChannelzServer {
	return &serverImpl{}
}

type serverImpl struct{}

func connectivityStateToProto(s connectivity.State) *pb.ChannelConnectivityState {
	switch s {
	case connectivity.Idle:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_IDLE}
	case connectivity.Connecting:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_CONNECTING}
	case connectivity.Ready:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_READY}
	case connectivity.TransientFailure:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_TRANSIENT_FAILURE}
	case connectivity.Shutdown:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_SHUTDOWN}
	default:
		return &pb.ChannelConnectivityState{State: pb.ChannelConnectivityState_UNKNOWN}
	}
}

func channelMetricToProto(cm *channelz.ChannelMetric) *pb.Channel {
	c := &pb.Channel{}
	c.Ref = &pb.ChannelRef{ChannelId: cm.ID, Name: cm.RefName}

	c.Data = &pb.ChannelData{
		State:          connectivityStateToProto(cm.ChannelData.State),
		Target:         cm.ChannelData.Target,
		CallsStarted:   cm.ChannelData.CallsStarted,
		CallsSucceeded: cm.ChannelData.CallsSucceeded,
		CallsFailed:    cm.ChannelData.CallsFailed,
	}
	if ts, err := ptypes.TimestampProto(cm.ChannelData.LastCallStartedTimestamp); err == nil {
		c.Data.LastCallStartedTimestamp = ts
	}
	nestedChans := make([]*pb.ChannelRef, 0, len(cm.NestedChans))
	for id, ref := range cm.NestedChans {
		nestedChans = append(nestedChans, &pb.ChannelRef{ChannelId: id, Name: ref})
	}
	c.ChannelRef = nestedChans

	subChans := make([]*pb.SubchannelRef, 0, len(cm.SubChans))
	for id, ref := range cm.SubChans {
		subChans = append(subChans, &pb.SubchannelRef{SubchannelId: id, Name: ref})
	}
	c.SubchannelRef = subChans

	sockets := make([]*pb.SocketRef, 0, len(cm.Sockets))
	for id, ref := range cm.Sockets {
		sockets = append(sockets, &pb.SocketRef{SocketId: id, Name: ref})
	}
	c.SocketRef = sockets
	return c
}

func subChannelMetricToProto(cm *channelz.SubChannelMetric) *pb.Subchannel {
	sc := &pb.Subchannel{}
	sc.Ref = &pb.SubchannelRef{SubchannelId: cm.ID, Name: cm.RefName}

	sc.Data = &pb.ChannelData{
		State:          connectivityStateToProto(cm.ChannelData.State),
		Target:         cm.ChannelData.Target,
		CallsStarted:   cm.ChannelData.CallsStarted,
		CallsSucceeded: cm.ChannelData.CallsSucceeded,
		CallsFailed:    cm.ChannelData.CallsFailed,
	}
	if ts, err := ptypes.TimestampProto(cm.ChannelData.LastCallStartedTimestamp); err == nil {
		sc.Data.LastCallStartedTimestamp = ts
	}
	nestedChans := make([]*pb.ChannelRef, 0, len(cm.NestedChans))
	for id, ref := range cm.NestedChans {
		nestedChans = append(nestedChans, &pb.ChannelRef{ChannelId: id, Name: ref})
	}
	sc.ChannelRef = nestedChans

	subChans := make([]*pb.SubchannelRef, 0, len(cm.SubChans))
	for id, ref := range cm.SubChans {
		subChans = append(subChans, &pb.SubchannelRef{SubchannelId: id, Name: ref})
	}
	sc.SubchannelRef = subChans

	sockets := make([]*pb.SocketRef, 0, len(cm.Sockets))
	for id, ref := range cm.Sockets {
		sockets = append(sockets, &pb.SocketRef{SocketId: id, Name: ref})
	}
	sc.SocketRef = sockets
	return sc
}

func addrToProto(a net.Addr) *pb.Address {
	switch a.Network() {
	case "udp":
		// TODO: Address_OtherAddress{}. Need proto def for Value.
	case "ip":
		// Note zone info is discarded through the conversion.
		return &pb.Address{Address: &pb.Address_TcpipAddress{TcpipAddress: &pb.Address_TcpIpAddress{IpAddress: a.(*net.IPAddr).IP}}}
	case "ip+net":
		// Note mask info is discarded through the conversion.
		return &pb.Address{Address: &pb.Address_TcpipAddress{TcpipAddress: &pb.Address_TcpIpAddress{IpAddress: a.(*net.IPNet).IP}}}
	case "tcp":
		// Note zone info is discarded through the conversion.
		return &pb.Address{Address: &pb.Address_TcpipAddress{TcpipAddress: &pb.Address_TcpIpAddress{IpAddress: a.(*net.TCPAddr).IP, Port: int32(a.(*net.TCPAddr).Port)}}}
	case "unix", "unixgram", "unixpacket":
		return &pb.Address{Address: &pb.Address_UdsAddress_{UdsAddress: &pb.Address_UdsAddress{Filename: a.String()}}}
	default:
	}
	return &pb.Address{}
}

func socketMetricToProto(sm *channelz.SocketMetric) *pb.Socket {
	s := &pb.Socket{}
	s.Ref = &pb.SocketRef{SocketId: sm.ID, Name: sm.RefName}

	s.Data = &pb.SocketData{
		StreamsStarted:   sm.SocketData.StreamsStarted,
		StreamsSucceeded: sm.SocketData.StreamsSucceeded,
		StreamsFailed:    sm.SocketData.StreamsFailed,
		MessagesSent:     sm.SocketData.MessagesSent,
		MessagesReceived: sm.SocketData.MessagesReceived,
		KeepAlivesSent:   sm.SocketData.KeepAlivesSent,
	}
	if ts, err := ptypes.TimestampProto(sm.SocketData.LastLocalStreamCreatedTimestamp); err == nil {
		s.Data.LastLocalStreamCreatedTimestamp = ts
	}
	if ts, err := ptypes.TimestampProto(sm.SocketData.LastRemoteStreamCreatedTimestamp); err == nil {
		s.Data.LastRemoteStreamCreatedTimestamp = ts
	}
	if ts, err := ptypes.TimestampProto(sm.SocketData.LastMessageSentTimestamp); err == nil {
		s.Data.LastMessageSentTimestamp = ts
	}
	if ts, err := ptypes.TimestampProto(sm.SocketData.LastMessageReceivedTimestamp); err == nil {
		s.Data.LastMessageReceivedTimestamp = ts
	}
	s.Data.LocalFlowControlWindow = &wrpb.Int64Value{Value: sm.SocketData.LocalFlowControlWindow}
	s.Data.RemoteFlowControlWindow = &wrpb.Int64Value{Value: sm.SocketData.RemoteFlowControlWindow}

	if sm.SocketData.LocalAddr != nil {
		s.Local = addrToProto(sm.SocketData.LocalAddr)
	}
	if sm.SocketData.RemoteAddr != nil {
		s.Remote = addrToProto(sm.SocketData.RemoteAddr)
	}
	s.RemoteName = sm.SocketData.RemoteName
	return s
}

func (s *serverImpl) GetTopChannels(ctx context.Context, req *pb.GetTopChannelsRequest) (*pb.GetTopChannelsResponse, error) {
	metrics, end := channelz.GetTopChannels(req.GetStartChannelId())
	resp := &pb.GetTopChannelsResponse{}
	for _, m := range metrics {
		resp.Channel = append(resp.Channel, channelMetricToProto(m))
	}
	resp.End = end
	return resp, nil
}

func serverMetricToProto(sm *channelz.ServerMetric) *pb.Server {
	s := &pb.Server{}
	s.Ref = &pb.ServerRef{ServerId: sm.ID, Name: sm.RefName}

	s.Data = &pb.ServerData{
		CallsStarted:   sm.ServerData.CallsStarted,
		CallsSucceeded: sm.ServerData.CallsSucceeded,
		CallsFailed:    sm.ServerData.CallsFailed,
	}

	if ts, err := ptypes.TimestampProto(sm.ServerData.LastCallStartedTimestamp); err == nil {
		s.Data.LastCallStartedTimestamp = ts
	}
	sockets := make([]*pb.SocketRef, 0, len(sm.ListenSockets))
	for id, ref := range sm.ListenSockets {
		sockets = append(sockets, &pb.SocketRef{SocketId: id, Name: ref})
	}
	s.ListenSocket = sockets
	return s
}

func (s *serverImpl) GetServers(ctx context.Context, req *pb.GetServersRequest) (*pb.GetServersResponse, error) {
	metrics, end := channelz.GetServers(req.GetStartServerId())
	resp := &pb.GetServersResponse{}
	for _, m := range metrics {
		resp.Server = append(resp.Server, serverMetricToProto(m))
	}
	resp.End = end
	return resp, nil
}

func (s *serverImpl) GetServerSockets(ctx context.Context, req *pb.GetServerSocketsRequest) (*pb.GetServerSocketsResponse, error) {
	metrics, end := channelz.GetServerSockets(req.GetServerId(), req.GetStartSocketId())
	resp := &pb.GetServerSocketsResponse{}
	for _, m := range metrics {
		resp.SocketRef = append(resp.SocketRef, &pb.SocketRef{SocketId: m.ID, Name: m.RefName})
	}
	resp.End = end
	return resp, nil
}

func (s *serverImpl) GetChannel(ctx context.Context, req *pb.GetChannelRequest) (*pb.GetChannelResponse, error) {
	var metric *channelz.ChannelMetric
	if metric = channelz.GetChannel(req.GetChannelId()); metric == nil {
		return &pb.GetChannelResponse{}, nil
	}
	resp := &pb.GetChannelResponse{Channel: channelMetricToProto(metric)}
	return resp, nil
}

func (s *serverImpl) GetSubchannel(ctx context.Context, req *pb.GetSubchannelRequest) (*pb.GetSubchannelResponse, error) {
	var metric *channelz.SubChannelMetric
	if metric = channelz.GetSubChannel(req.GetSubchannelId()); metric == nil {
		return &pb.GetSubchannelResponse{}, nil
	}
	resp := &pb.GetSubchannelResponse{Subchannel: subChannelMetricToProto(metric)}
	return resp, nil
}

func (s *serverImpl) GetSocket(ctx context.Context, req *pb.GetSocketRequest) (*pb.GetSocketResponse, error) {
	var metric *channelz.SocketMetric
	if metric = channelz.GetSocket(req.GetSocketId()); metric == nil {
		return &pb.GetSocketResponse{}, nil
	}
	resp := &pb.GetSocketResponse{Socket: socketMetricToProto(metric)}
	return resp, nil
}
