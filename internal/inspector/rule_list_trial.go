//go:build plugin_trial
// +build plugin_trial

package inspector

import (
	"github.com/actiontech/sqle-pg-plugin/plocale"
	"github.com/actiontech/sqle/sqle/pkg/params"

	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
)

const dbTypePG = "PostgreSQL"

var sourceRuleHandlers = []SourceHandler{
	{
		Rule: SourceRule{
			Name:       RuleId1,
			Desc:       plocale.PgRule001Desc,
			Annotation: plocale.PgRule001Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule001Message,
		AstSQLHandler:        rule1,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId2,
			Desc:       plocale.PgRule002Desc,
			Annotation: plocale.PgRule002Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule002Message,
		AstSQLHandler:        rule2,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId3,
			Desc:       plocale.PgRule003Desc,
			Annotation: plocale.PgRule003Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule003Message,
		AstSQLHandler:        rule3,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId4,
			Desc:       plocale.PgRule004Desc,
			Annotation: plocale.PgRule004Annotation,
			Category:   plocale.PgRuleTypeSuggestion,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule004Message,
		AstSQLHandler:        rule4,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId5,
			Desc:       plocale.PgRule005Desc,
			Annotation: plocale.PgRule005Annotation,
			Category:   plocale.PgRuleTypeSuggestion,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule005Message,
		AstSQLHandler:        rule5,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId6,
			Desc:       plocale.PgRule006Desc,
			Annotation: plocale.PgRule006Annotation,
			Category:   plocale.PgRuleTypeSuggestion,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule006Message,
		AstSQLHandler:        rule6,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId7,
			Desc:       plocale.PgRule007Desc,
			Annotation: plocale.PgRule007Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule007Message,
		AstSQLHandler:        rule7,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId9,
			Desc:       plocale.PgRule009Desc,
			Annotation: plocale.PgRule009Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelWarn,
			Params: []*SourceParam{
				{Key: "max_column_count", Value: "50", Desc: plocale.PgRule009Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule009Message,
		AstSQLHandler:        rule9,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId10,
			Desc:       plocale.PgRule010Desc,
			Annotation: plocale.PgRule010Annotation,
			Category:   plocale.PgRuleTypeSuggestion,
			Level:      driverV2.RuleLevelError,
			Params: []*SourceParam{
				{Key: "sql_length", Value: "1024", Desc: plocale.PgRule010Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule010Message,
		RawSQLHandler:        rule10,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId12,
			Desc:       plocale.PgRule012Desc,
			Annotation: plocale.PgRule012Annotation,
			Category:   plocale.PgRuleTypeNamingConvention,
			Level:      driverV2.RuleLevelError,
			Params: []*SourceParam{
				{Key: "expect_index_name_prefix", Value: "idx_", Desc: plocale.PgRule012Params1, Type: params.ParamTypeString},
			},
		},
		Message:              plocale.PgRule012Message,
		AstSQLHandler:        rule12,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId15,
			Desc:       plocale.PgRule015Desc,
			Annotation: plocale.PgRule015Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule015Message,
		AstSQLHandler:        rule15,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId16,
			Desc:       plocale.PgRule016Desc,
			Annotation: plocale.PgRule016Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule016Message,
		AstSQLHandler:        rule16,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId17,
			Desc:       plocale.PgRule017Desc,
			Annotation: plocale.PgRule017Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelWarn,
			Params: []*SourceParam{
				{Key: "expect_number_of_scan_line", Value: "10000", Desc: plocale.PgRule017Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule017Message,
		RawSQLHandler:        rule17,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId21,
			Desc:       plocale.PgRule021Desc,
			Annotation: plocale.PgRule021Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:       plocale.PgRule021Message,
		AstSQLHandler: rule21,
		AllowOffline:  true,
	},
	{
		Rule: SourceRule{
			Name:       RuleId26,
			Desc:       plocale.PgRule026Desc,
			Annotation: plocale.PgRule026Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelWarn,
		},
		Message:       plocale.PgRule026Message,
		AstSQLHandler: rule26,
		AllowOffline:  true,
	},
}

func init() {
	RuleHandlers = GenerateI18nRuleHandlers(plocale.Bundle, sourceRuleHandlers, dbTypePG)
	for _, rh := range RuleHandlers {
		RuleHandlerMap[rh.Rule.Name] = rh
	}
}
