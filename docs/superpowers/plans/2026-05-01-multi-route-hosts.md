# Multi Route Hosts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-domain service routing while keeping `routeHost` as the primary compatible host.

**Architecture:** Store full host lists in `Service.RouteHosts` JSONB and keep `Service.RouteHost` as the primary host. Normalize and validate host lists in the service catalog application layer, propagate them through gRPC proxy snapshots, and make the agent generate multi-value HAProxy host ACLs.

**Tech Stack:** Go, GORM, gin DTOs, protobuf/gRPC, HAProxy Data Plane config structs, React, TypeScript, react-hook-form, zod.

---

### Task 1: Backend Service Catalog Host Lists

**Files:**
- Modify: `internal/shared/model/entities.go`
- Modify: `internal/shared/model/fx.go`
- Modify: `internal/shared/dto/service.go`
- Modify: `internal/servicecatalog/domain/repository.go`
- Modify: `internal/servicecatalog/infra/repository.go`
- Modify: `internal/servicecatalog/application/service.go`
- Modify: `internal/servicecatalog/application/proxy_config.go`
- Test: `internal/servicecatalog/application/service_test.go`

- [ ] **Step 1: Add failing service catalog tests**

Add tests that create multi-host services, verify normalization and fallback behavior, and reject conflicts on any host.

- [ ] **Step 2: Run service catalog tests and verify failure**

Run: `go test ./internal/servicecatalog/application`

Expected: FAIL because `RouteHosts` fields and multi-host behavior are missing.

- [ ] **Step 3: Implement service catalog fields, normalization, repository port, and backfill**

Add `RouteHosts` JSONB to the model and DTOs, normalize route host lists in `buildServiceEntity`, update output/spec/proxy config mapping, add a repository method for multi-host conflict lookup, and backfill empty `route_hosts` from `route_host` during model migration.

- [ ] **Step 4: Run service catalog tests and verify pass**

Run: `go test ./internal/servicecatalog/application ./internal/servicecatalog/infra ./internal/shared/model`

Expected: PASS.

### Task 2: gRPC Proxy Snapshot Propagation

**Files:**
- Modify: `internal/shared/grpcapi/agent_control.proto`
- Modify: `internal/shared/grpcapi/agent_control.pb.go`
- Modify: `adapter/grpc/controlplane/proxy_config.go`
- Test: `adapter/grpc/controlplane/proxy_config_test.go`

- [ ] **Step 1: Add failing proxy snapshot test**

Add a test asserting `ProxyServiceConfig.RouteHosts` is populated and sorted with the service config.

- [ ] **Step 2: Run control-plane proxy tests and verify failure**

Run: `go test ./adapter/grpc/controlplane`

Expected: FAIL because protobuf and snapshot mapping do not expose `RouteHosts`.

- [ ] **Step 3: Add protobuf field and regenerate Go code**

Add `repeated string route_hosts = 17;` to `ProxyServiceConfig`, regenerate `agent_control.pb.go`, and map `RouteHosts` in `buildProxyConfigSnapshot`.

- [ ] **Step 4: Run control-plane proxy tests and verify pass**

Run: `go test ./adapter/grpc/controlplane`

Expected: PASS.

### Task 3: Agent HAProxy Multi-Host ACLs

**Files:**
- Modify: `internal/agent/infra/runtime/proxy_stack.go`
- Test: `internal/agent/infra/runtime/proxy_stack_test.go`

- [ ] **Step 1: Add failing agent proxy tests**

Add tests that assert frontend host ACL and normalize-cookie return conditions include all route hosts.

- [ ] **Step 2: Run agent runtime tests and verify failure**

Run: `go test ./internal/agent/infra/runtime`

Expected: FAIL because only `route_host` is used.

- [ ] **Step 3: Implement multi-host ACL helpers**

Add a helper that returns normalized unique hosts from `route_hosts` or falls back to `route_host`. Use it in frontend ACL generation, normalize return conditions, sorting, and snapshot cloning.

- [ ] **Step 4: Run agent runtime tests and verify pass**

Run: `go test ./internal/agent/infra/runtime`

Expected: PASS.

### Task 4: Frontend Service Form and Display

**Files:**
- Modify: `web/default/src/features/services/types.ts`
- Modify: `web/default/src/shared/lib/forms.ts`
- Modify: `web/default/src/features/services/components/ServiceForm.tsx`
- Modify: `web/default/src/features/services/components/ServicesPage.tsx`

- [ ] **Step 1: Add frontend types and form mapping changes**

Add `routeHosts` and `routeHostsText`, parse multi-line hosts, default from `service.routeHosts ?? [service.routeHost]`, and send both primary and list fields.

- [ ] **Step 2: Update service form UI**

Add a multi-line access domains field next to the primary route host field using existing form styles and concise hints.

- [ ] **Step 3: Update services list display**

Show the primary route and compactly indicate additional domain count when `routeHosts.length > 1`.

- [ ] **Step 4: Run frontend type/build checks**

Run in `web/default`: `npm install` if dependencies are absent, then `npm run build`.

Expected: PASS.

### Task 5: Formatting and Final Verification

**Files:**
- All changed Go and TypeScript files.

- [ ] **Step 1: Format Go files**

Run: `goimports -w <changed-go-files>`

- [ ] **Step 2: Run targeted Go tests**

Run: `go test ./internal/servicecatalog/application ./adapter/grpc/controlplane ./internal/agent/infra/runtime`

Expected: PASS.

- [ ] **Step 3: Build frontend assets**

Run in `web/default`: `npm run build`

Expected: PASS and `web/default/dist` exists for Go embed.

- [ ] **Step 4: Run full Go test suite**

Run: `go test ./...`

Expected: PASS.
