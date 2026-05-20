//go:build plugin_trial
// +build plugin_trial

package inspector

import (
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
)

var RuleHandlers = []RuleHandler{}
