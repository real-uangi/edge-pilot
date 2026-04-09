package application

import (
	"edge-pilot/internal/shared/model"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	SharedFrontendName     = "ep_http"
	SharedDefaultBackend   = "ep_default"
	SharedFrontendBindPort = 80
)

type ProxyServiceConfig struct {
	ServiceID       uuid.UUID
	ServiceKey      string
	RouteHost       string
	RoutePathPrefix string
	BackendName     string
	CurrentLiveSlot model.Slot
	BlueHostPort    int
	GreenHostPort   int
}

func NormalizeRouteHost(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeRoutePathPrefix(value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
	}
	return path
}

func BackendName(serviceID uuid.UUID) string {
	return serviceID.String()
}

func ServerName(slot model.Slot) string {
	switch slot {
	case model.SlotBlue:
		return "blue"
	case model.SlotGreen:
		return "green"
	default:
		return ""
	}
}

func BuildProxyServiceConfigs(services []model.Service) []ProxyServiceConfig {
	out := make([]ProxyServiceConfig, 0, len(services))
	for _, item := range services {
		if item.Enabled == nil || !*item.Enabled {
			continue
		}
		out = append(out, ProxyServiceConfig{
			ServiceID:       item.ID,
			ServiceKey:      item.ServiceKey,
			RouteHost:       NormalizeRouteHost(item.RouteHost),
			RoutePathPrefix: NormalizeRoutePathPrefix(item.RoutePathPrefix),
			BackendName:     BackendName(item.ID),
			CurrentLiveSlot: item.CurrentLiveSlot,
			BlueHostPort:    item.BlueHostPort,
			GreenHostPort:   item.GreenHostPort,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RouteHost != out[j].RouteHost {
			return out[i].RouteHost < out[j].RouteHost
		}
		if len(out[i].RoutePathPrefix) != len(out[j].RoutePathPrefix) {
			return len(out[i].RoutePathPrefix) > len(out[j].RoutePathPrefix)
		}
		return out[i].ServiceKey < out[j].ServiceKey
	})
	return out
}
