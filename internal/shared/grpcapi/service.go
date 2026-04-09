package grpcapi

import (
	"context"

	"google.golang.org/grpc"
)

const AgentControlServiceName = "edgepilot.AgentControl"

type AgentControlClient interface {
	Connect(ctx context.Context, opts ...grpc.CallOption) (AgentControl_ConnectClient, error)
}

type AgentControl_ConnectClient interface {
	Send(*AgentMessage) error
	Recv() (*ControlMessage, error)
	grpc.ClientStream
}

type AgentControlServer interface {
	Connect(AgentControl_ConnectServer) error
}

type AgentControl_ConnectServer interface {
	Send(*ControlMessage) error
	Recv() (*AgentMessage, error)
	grpc.ServerStream
}

type agentControlClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentControlClient(cc grpc.ClientConnInterface) AgentControlClient {
	return &agentControlClient{cc: cc}
}

func (c *agentControlClient) Connect(ctx context.Context, opts ...grpc.CallOption) (AgentControl_ConnectClient, error) {
	stream, err := c.cc.NewStream(ctx, &AgentControl_ServiceDesc.Streams[0], "/"+AgentControlServiceName+"/Connect", opts...)
	if err != nil {
		return nil, err
	}
	return &agentControlConnectClient{ClientStream: stream}, nil
}

type agentControlConnectClient struct {
	grpc.ClientStream
}

func (c *agentControlConnectClient) Send(msg *AgentMessage) error {
	return c.ClientStream.SendMsg(msg)
}

func (c *agentControlConnectClient) Recv() (*ControlMessage, error) {
	msg := new(ControlMessage)
	if err := c.ClientStream.RecvMsg(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func RegisterAgentControlServer(registrar grpc.ServiceRegistrar, srv AgentControlServer) {
	registrar.RegisterService(&AgentControl_ServiceDesc, srv)
}

var AgentControl_ServiceDesc = grpc.ServiceDesc{
	ServiceName: AgentControlServiceName,
	HandlerType: (*AgentControlServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Connect",
			Handler:       _AgentControl_Connect_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

func _AgentControl_Connect_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(AgentControlServer).Connect(&agentControlConnectServer{ServerStream: stream})
}

type agentControlConnectServer struct {
	grpc.ServerStream
}

func (s *agentControlConnectServer) Send(msg *ControlMessage) error {
	return s.ServerStream.SendMsg(msg)
}

func (s *agentControlConnectServer) Recv() (*AgentMessage, error) {
	msg := new(AgentMessage)
	if err := s.ServerStream.RecvMsg(msg); err != nil {
		return nil, err
	}
	return msg, nil
}
