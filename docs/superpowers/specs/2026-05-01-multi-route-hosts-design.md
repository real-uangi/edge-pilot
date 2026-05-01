# Multi Route Hosts Design

## Goal

Services support multiple access domains while preserving the existing primary `routeHost` contract. All domains for a service share the same `routePathPrefix`, release state, traffic splitting, sticky cookie behavior, and backend targets.

## Architecture

Add a `routeHosts` list alongside the existing `routeHost` field. `routeHost` remains the primary host for compatibility, display, and verification URLs. `routeHosts` stores the complete normalized host list and is used by proxy config generation.

## Data Model

`ep_service` gains a JSONB `route_hosts` column containing `[]string`. Existing `route_host` remains indexed and not null. Startup migration backfills empty `route_hosts` values from `route_host` so old rows behave as single-host services.

## API Behavior

Admin create/update accepts both `routeHost` and `routeHosts`.

- If `routeHosts` is empty, the service uses `[routeHost]`.
- If `routeHost` is empty but `routeHosts` has values, the first normalized host becomes the primary host.
- If both are present, the primary `routeHost` is included in the list.
- Hosts are trimmed, lowercased, deduplicated, and empty values are ignored.

Responses return both fields. Existing callers can continue reading `routeHost`; new callers should use `routeHosts` for full routing configuration.

## Validation

For each service, `(agentId, routePathPrefix, host)` must be unique across services. Create and update reject a request if any normalized host in `routeHosts` conflicts with another service on the same agent and path prefix.

## Proxy Data Flow

The service catalog builds `ProxyServiceConfig` with both `RouteHost` and `RouteHosts`. The control-plane gRPC snapshot sends both fields. Agents prefer `route_hosts`; if it is empty, they fall back to `route_host`.

HAProxy host ACLs use a multi-value case-insensitive host match, such as:

```text
hdr(host) -i api.example.com api-alt.example.com
```

Normalize-cookie return rules use the same host list, so all service domains share release stickiness and preview behavior.

## Frontend

The service form keeps a primary domain input and adds a multi-line access domains input. Defaults use `routeHosts` when available and fall back to `routeHost`. Payloads send both `routeHost` and `routeHosts`. The services table continues to show the primary route and adds a compact count when multiple domains are configured.

## Testing

Tests cover route host normalization, duplicate conflict rejection, single-host compatibility, proxy snapshot propagation, agent HAProxy ACL generation, and frontend type/build checks.
