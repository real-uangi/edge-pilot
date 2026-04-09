package bootstrap

import (
	"edge-pilot/internal/shared/buildinfo"

	"github.com/real-uangi/allingo/common/log"
)

var bootstrapLogger = log.NewStdLogger("bootstrap")

func logBuildInfo(role string) {
	bootstrapLogger.Infof(
		"Starting %s with build info: version=%s commit=%s build_time=%s",
		role,
		buildinfo.Version,
		buildinfo.Commit,
		buildinfo.BuildTime,
	)
}
