package config

import "go.uber.org/fx"

var ControlPlaneModule = fx.Module(
	"shared-config-control-plane",
	fx.Provide(
		LoadAgentAuthConfig,
		LoadSchedulerConfig,
		LoadAdminAuthConfig,
		LoadRegistryCredentialConfig,
		LoadServiceSecretConfig,
	),
)
