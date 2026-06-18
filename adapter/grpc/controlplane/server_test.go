package controlplane

import (
	"context"
	"io"
	"testing"
	"time"

	agentregistry "github.com/real-uangi/edge-pilot/internal/agent/application/registry"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"

	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"google.golang.org/grpc/metadata"
)

func TestConnectRejectsInvalidToken(t *testing.T) {
	auth := config.LoadAgentAuthConfig()
	_, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	enabled := true
	registry := agentregistry.NewRegistryService(auth, &fakeAgentRepo{
		nodes: map[string]*model.AgentNode{
			"11111111-1111-1111-1111-111111111111": {
				ID:        "11111111-1111-1111-1111-111111111111",
				TokenHash: hash,
				Enabled:   &enabled,
			},
		},
	})

	server := NewServer(NewSessionHub(nil), registry, nil, nil, nil)
	stream := &fakeStream{
		recvMessages: []*grpcapi.AgentMessage{
			{
				Payload: &grpcapi.AgentMessage_Hello{
					Hello: &grpcapi.HelloMessage{
						AgentId: "11111111-1111-1111-1111-111111111111",
						Token:   "bad-token",
					},
				},
			},
		},
	}

	err = server.Connect(stream)
	if err == nil {
		t.Fatalf("expected auth error")
	}
}

func TestAgentSessionSendReturnsOfflineWhenQueueFull(t *testing.T) {
	session := &agentSession{sendCh: make(chan *grpcapi.ControlMessage, 1)}
	session.sendCh <- &grpcapi.ControlMessage{}

	if err := session.send(&grpcapi.ControlMessage{}); err == nil {
		t.Fatalf("expected send queue full error")
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

type fakeAgentRepo struct {
	nodes map[string]*model.AgentNode
}

func (r *fakeAgentRepo) Save(*model.AgentNode) error { return nil }
func (r *fakeAgentRepo) Get(id string) (*model.AgentNode, error) {
	if r.nodes == nil {
		return nil, nil
	}
	return r.nodes[id], nil
}
func (r *fakeAgentRepo) Delete(string) error              { return nil }
func (r *fakeAgentRepo) List() ([]model.AgentNode, error) { return nil, nil }
func (r *fakeAgentRepo) ListEnabled() ([]model.AgentNode, error) {
	return nil, nil
}
func (r *fakeAgentRepo) MarkOffline(string, string) error { return nil }
func (r *fakeAgentRepo) MarkOfflineStale(time.Time) ([]string, error) {
	return nil, nil
}
