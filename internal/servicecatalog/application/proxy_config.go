package application

import (
	"edge-pilot/internal/shared/model"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	SharedFrontendName         = "ep_http"
	SharedDefaultBackend       = "ep_default"
	SharedFrontendBindPort     = 80
	PreviewReleaseIDQueryParam = "__ep_release_id"
	CurrentReleaseIDHeaderName = "X-Edge-Pilot-Current-Release-Id"
	LiveReleaseIDHeaderName    = "X-Edge-Pilot-Live-Release-Id"
	StickyCookieMaxAgeSec      = 600
)

type ProxyServiceConfig struct {
	ServiceID       uuid.UUID
	ServiceKey      string
	RouteHost       string
	RoutePathPrefix string
	BackendName     string
	CurrentLiveSlot model.Slot
	ContainerPort   int
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

func BackendNameForSlot(base string, slot model.Slot) string {
	return base + "_" + SlotToken(slot)
}

func StickyCookieName(serviceKey string) string {
	normalized := strings.ToLower(strings.TrimSpace(serviceKey))
	if normalized == "" {
		normalized = "service"
	}
	var builder strings.Builder
	builder.Grow(len(normalized))
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return "ep_release_id_" + strings.Trim(builder.String(), "_")
}

func SlotToken(slot model.Slot) string {
	switch slot {
	case model.SlotBlue:
		return "blue"
	case model.SlotGreen:
		return "green"
	default:
		return ""
	}
}

func BuildVerificationURL(routeHost string, routePathPrefix string, releaseID string) string {
	host := NormalizeRouteHost(routeHost)
	releaseID = strings.TrimSpace(releaseID)
	if host == "" || releaseID == "" {
		return ""
	}
	values := url.Values{}
	values.Set(PreviewReleaseIDQueryParam, releaseID)
	return "//" + host + NormalizeRoutePathPrefix(routePathPrefix) + "?" + values.Encode()
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
			ContainerPort:   item.ContainerPort,
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

func BuildStickyCookie(cookieName string, releaseID string, routePathPrefix string) string {
	releaseID = strings.TrimSpace(releaseID)
	cookieName = strings.TrimSpace(cookieName)
	if releaseID == "" || cookieName == "" {
		return ""
	}
	path := NormalizeRoutePathPrefix(routePathPrefix)
	parts := []string{
		cookieName + "=" + releaseID,
		"Max-Age=" + strconv.Itoa(StickyCookieMaxAgeSec),
		"Path=" + path,
		"HttpOnly",
		"SameSite=Lax",
	}
	return strings.Join(parts, "; ")
}
