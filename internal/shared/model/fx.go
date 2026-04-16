package model

import (
	"github.com/real-uangi/allingo/common/db"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var ControlPlaneModule = fx.Module(
	"shared-model",
	fx.Invoke(autoMigrate),
)

func autoMigrate(manager *db.Manager) error {
	conn := manager.GetDB()
	if err := conn.AutoMigrate(
		&Service{},
		&RegistryCredential{},
		&Release{},
		&Task{},
		&TaskAttempt{},
		&AgentNode{},
		&RuntimeInstance{},
		&AuditLog{},
		&BackendStatSnapshot{},
	); err != nil {
		return err
	}
	return backfillServiceHealthConfig(conn)
}

func backfillServiceHealthConfig(conn *gorm.DB) error {
	updates := []struct {
		column string
		value  any
	}{
		{column: "startup_grace_second", value: DefaultStartupGraceSecond},
		{column: "http_probe_timeout_second", value: DefaultHTTPProbeTimeoutSecond},
		{column: "http_probe_interval_second", value: DefaultHTTPProbeIntervalSecond},
		{column: "http_success_threshold", value: DefaultHTTPSuccessThreshold},
	}
	for _, item := range updates {
		if err := conn.Model(&Service{}).
			Where(item.column+" = ?", 0).
			Update(item.column, item.value).Error; err != nil {
			return err
		}
	}
	if err := conn.Model(&Service{}).
		Where("http_timeout_second IN ?", []int{0, 5}).
		Update("http_timeout_second", DefaultHTTPTimeoutSecond).Error; err != nil {
		return err
	}
	if err := conn.Model(&Task{}).
		Where("cleanup_completed IS NULL").
		Update("cleanup_completed", false).Error; err != nil {
		return err
	}
	return nil
}
