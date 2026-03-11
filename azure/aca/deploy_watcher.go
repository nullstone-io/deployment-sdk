package aca

import (
	"github.com/nullstone-io/deployment-sdk/app"
)

var NewDeployWatcher = app.NewPollingDeployWatcher(NewDeployStatusGetter)
