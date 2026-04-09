package model

import (
	"github.com/real-uangi/allingo/common/db"
	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"shared-model",
	fx.Invoke(autoMigrate),
)

func autoMigrate(manager *db.Manager) error {
	return manager.GetDB().AutoMigrate(
		&Service{},
		&Release{},
		&Task{},
		&TaskAttempt{},
		&AgentNode{},
		&RuntimeInstance{},
		&AuditLog{},
		&BackendStatSnapshot{},
	)
}
