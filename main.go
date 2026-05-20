package main

import (
	"flag"
	"fmt"

	"github.com/actiontech/sqle-pg-plugin/internal"
	"github.com/actiontech/sqle-pg-plugin/internal/inspector"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
)

var version string

var printVersion = flag.Bool("version", false, "Print version & exit")

func main() {
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		return
	}

	builder := driverPkg.NewDriverBuilder(&internal.Dialector{})
	builder.SetEnableOptionalModule(
		driverV2.OptionalModuleQuery,
		driverV2.OptionalModuleExplain,
		driverV2.OptionalModuleExtractTableFromSQL,
		driverV2.OptionalModuleGetTableMeta,
		driverV2.OptionalModuleEstimateSQLAffectRows,
		driverV2.OptionalModuleGenRollbackSQL,
		driverV2.OptionalModuleKillProcess,
		driverV2.OptionalModuleI18n)
	builder.Meta.DatabaseDefaultPort = 5432
	builder.Meta.PluginName = "GaussDB"
	builder.Meta.Logo = logo

	builder.SetSQLParserFn(inspector.SqlParserFunc)
	for _, rule := range inspector.RuleHandlers {
		r := rule
		if r.AstSQLHandler != nil {
			builder.AddRuleWithSQLParser(&r.Rule, r.AstSQLHandler)
		}
		if r.RawSQLHandler != nil {
			builder.AddRule(&r.Rule, r.RawSQLHandler)
		}

		// 无 handler 的全局配置规则(RuleId24,RuleId25)，直接添加
		if rule.Rule.Name == inspector.RuleId24 || rule.Rule.Name == inspector.RuleId25 {
			builder.AddRule(&r.Rule, nil)
		}
	}

	builder.Serve(inspector.NewDriverImpl)
}
