package infra

import (
	"bufio"
	"context"
	"edge-pilot/internal/shared/grpcapi"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type HAProxyRuntimeClient struct {
	resolveAddress func() string
}

func newHAProxyRuntimeClient(resolveAddress func() string) *HAProxyRuntimeClient {
	return &HAProxyRuntimeClient{
		resolveAddress: resolveAddress,
	}
}

func (c *HAProxyRuntimeClient) SetServerAddress(ctx context.Context, backend string, server string, address string, port int) error {
	_, err := c.run(ctx, fmt.Sprintf("set server %s/%s addr %s port %d", backend, server, address, port))
	return err
}

func (c *HAProxyRuntimeClient) EnableServer(ctx context.Context, backend string, server string) error {
	_, err := c.run(ctx, fmt.Sprintf("enable server %s/%s", backend, server))
	return err
}

func (c *HAProxyRuntimeClient) DisableServer(ctx context.Context, backend string, server string) error {
	_, err := c.run(ctx, fmt.Sprintf("disable server %s/%s", backend, server))
	return err
}

func (c *HAProxyRuntimeClient) ShowStats(ctx context.Context) ([]*grpcapi.BackendStatPoint, error) {
	output, err := c.run(ctx, "show stat")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(output, "\n")
	stats := make([]*grpcapi.BackendStatPoint, 0)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 34 {
			continue
		}
		scur, _ := strconv.ParseInt(parts[4], 10, 64)
		rate, _ := strconv.ParseInt(parts[33], 10, 64)
		eresp, _ := strconv.ParseInt(parts[14], 10, 64)
		stats = append(stats, &grpcapi.BackendStatPoint{
			BackendName:   parts[0],
			ServerName:    parts[1],
			Scur:          scur,
			Rate:          rate,
			ErrorRequests: eresp,
		})
	}
	return stats, nil
}

func (c *HAProxyRuntimeClient) run(ctx context.Context, command string) (string, error) {
	address := strings.TrimSpace(c.resolveAddress())
	if address == "" {
		return "", fmt.Errorf("haproxy runtime address is empty")
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}
	if _, err := conn.Write([]byte(command + "\n")); err != nil {
		return "", err
	}
	reader := bufio.NewReader(conn)
	data, err := reader.ReadString('\x00')
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return "", err
	}
	return strings.TrimSuffix(data, "\x00"), nil
}
