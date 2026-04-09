package controlplane

import (
	"context"
	edgeagent "edge-pilot/internal/agent/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"io"
	"testing"
	"time"

	"edge-pilot/internal/shared/model"

	"google.golang.org/grpc/metadata"
)

func TestConnectRejectsInvalidToken(t *testing.T) {
	registry := edgeagent.NewRegistryService(&config.AgentAuthConfig{
		TokenByID: map[string]string{"agent-a": "valid-token"},
	}, &fakeAgentRepo{})

	server := NewServer(NewSessionHub(), registry, nil, nil, nil)
	stream := &fakeStream{
		recvMessages: []*grpcapi.AgentMessage{
			{
				Payload: &grpcapi.AgentMessage_Hello{
					Hello: &grpcapi.HelloMessage{
						AgentId: "agent-a",
						Token:   "bad-token",
					},
				},
			},
		},
	}

	err := server.Connect(stream)
	if err == nil {
		t.Fatalf("expected auth error")
	}
}

type fakeStream struct {
	recvMessages []*grpcapi.AgentMessage
	sentMessages []*grpcapi.ControlMessage
}

func (s *fakeStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeStream) SetTrailer(metadata.MD)       {}
func (s *fakeStream) Context() context.Context     { return context.Background() }
func (s *fakeStream) SendMsg(m interface{}) error  { return nil }
func (s *fakeStream) RecvMsg(m interface{}) error  { return nil }

func (s *fakeStream) Send(msg *grpcapi.ControlMessage) error {
	s.sentMessages = append(s.sentMessages, msg)
	return nil
}

func (s *fakeStream) Recv() (*grpcapi.AgentMessage, error) {
	if len(s.recvMessages) == 0 {
		return nil, io.EOF
	}
	msg := s.recvMessages[0]
	s.recvMessages = s.recvMessages[1:]
	return msg, nil
}

type fakeAgentRepo struct{}

func (r *fakeAgentRepo) Save(*model.AgentNode) error          { return nil }
func (r *fakeAgentRepo) Get(string) (*model.AgentNode, error) { return nil, nil }
func (r *fakeAgentRepo) List() ([]model.AgentNode, error)     { return nil, nil }
func (r *fakeAgentRepo) MarkOffline(string, string) error     { return nil }
func (r *fakeAgentRepo) MarkOfflineStale(time.Time) ([]string, error) {
	return nil, nil
}
