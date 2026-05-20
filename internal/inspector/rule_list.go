//go:build !plugin_trial
// +build !plugin_trial

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
			Name:       RuleId8,
			Desc:       plocale.PgRule008Desc,
			Annotation: plocale.PgRule008Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule008Message,
		AstSQLHandler:        rule8,
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
			Name:       RuleId11,
			Desc:       plocale.PgRule011Desc,
			Annotation: plocale.PgRule011Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelNotice,
			Params: []*SourceParam{
				{Key: "expect_max_index_column_number", Value: "3", Desc: plocale.PgRule011Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule011Message,
		AstSQLHandler:        rule11,
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
			Name:       RuleId19,
			Desc:       plocale.PgRule019Desc,
			Annotation: plocale.PgRule019Annotation,
			Category:   plocale.PgRuleTypeNamingConvention,
			Level:      driverV2.RuleLevelError,
			Params: []*SourceParam{
				{Key: "expected_nesting_layers", Value: "3", Desc: plocale.PgRule019Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule019Message,
		AstSQLHandler:        rule19,
		AllowOffline:         true,
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
			Name:       RuleId22,
			Desc:       plocale.PgRule022Desc,
			Annotation: plocale.PgRule022Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:       plocale.PgRule022Message,
		AstSQLHandler: rule22,
		AllowOffline:  false,
	},
	{
		Rule: SourceRule{
			Name:       RuleId23,
			Desc:       plocale.PgRule023Desc,
			Annotation: plocale.PgRule023Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:       plocale.PgRule023Message,
		AstSQLHandler: rule23,
		AllowOffline:  false,
	},
	{
		Rule: SourceRule{
			Name:       RuleId24,
			Desc:       plocale.PgRule024Desc,
			Annotation: plocale.PgRule024Annotation,
			Category:   plocale.PgRuleTypeGlobalConfig,
			Level:      driverV2.RuleLevelNotice,
			Params: []*SourceParam{
				{Key: "max_affected_rows", Value: "1000", Desc: plocale.PgRule024Params1, Type: params.ParamTypeInt},
			},
		},
		RawSQLHandler: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId25,
			Desc:       plocale.PgRule025Desc,
			Annotation: plocale.PgRule025Annotation,
			Category:   plocale.PgRuleTypeGlobalConfig,
			Level:      driverV2.RuleLevelNotice,
		},
		RawSQLHandler: nil,
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
		AllowOffline:  false,
	},
	{
		Rule: SourceRule{
			Name:       RuleId27,
			Desc:       plocale.PgRule027Desc,
			Annotation: plocale.PgRule027Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelWarn,
		},
		Message:              plocale.PgRule027Message,
		AstSQLHandler:        rule27,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId28,
			Desc:       plocale.PgRule028Desc,
			Annotation: plocale.PgRule028Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:              plocale.PgRule028Message,
		RawSQLHandler:        rule28,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId31,
			Desc:       plocale.PgRule031Desc,
			Annotation: plocale.PgRule031Annotation,
			Category:   plocale.PgRuleTypeIndexOptimization,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:       plocale.PgRule031Message,
		AstSQLHandler: rule31,
		AllowOffline:  false,
	},
	{
		Rule: SourceRule{
			Name:       RuleId32,
			Desc:       plocale.PgRule032Desc,
			Annotation: plocale.PgRule032Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:              plocale.PgRule032Message,
		AstSQLHandler:        rule32,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId33,
			Desc:       plocale.PgRule033Desc,
			Annotation: plocale.PgRule033Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:              plocale.PgRule033Message,
		AstSQLHandler:        rule33,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId34,
			Desc:       plocale.PgRule034Desc,
			Annotation: plocale.PgRule034Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:              plocale.PgRule034Message,
		AstSQLHandler:        rule34,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId35,
			Desc:       plocale.PgRule035Desc,
			Annotation: plocale.PgRule035Annotation,
			Category:   plocale.PgRuleTypeSuggestion,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:              plocale.PgRule035Message,
		AstSQLHandler:        rule35,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId36,
			Desc:       plocale.PgRule036Desc,
			Annotation: plocale.PgRule036Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelWarn,
		},
		Message:       plocale.PgRule036Message,
		AstSQLHandler: rule36,
		AllowOffline:  true,
	},
	{
		Rule: SourceRule{
			Name:       RuleId37,
			Desc:       plocale.PgRule037Desc,
			Annotation: plocale.PgRule037Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelWarn,
		},
		Message:       plocale.PgRule037Message,
		AstSQLHandler: rule37,
		AllowOffline:  true,
	},
	{
		Rule: SourceRule{
			Name:       RuleId38,
			Desc:       plocale.PgRule038Desc,
			Annotation: plocale.PgRule038Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelWarn,
		},
		Message:              plocale.PgRule038Message,
		AstSQLHandler:        rule38,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId39,
			Desc:       plocale.PgRule039Desc,
			Annotation: plocale.PgRule039Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelNotice,
			Params: []*SourceParam{
				{Key: "expect_max_index_column_number", Value: "5", Desc: plocale.PgRule039Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule039Message,
		AstSQLHandler:        rule39,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId40,
			Desc:       plocale.PgRule040Desc,
			Annotation: plocale.PgRule040Annotation,
			Category:   plocale.PgRuleTypeDDLConvention,
			Level:      driverV2.RuleLevelNotice,
		},
		Message:              plocale.PgRule040Message,
		AstSQLHandler:        rule40,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{
			Name:       RuleId41,
			Desc:       plocale.PgRule041Desc,
			Annotation: plocale.PgRule041Annotation,
			Category:   plocale.PgRuleTypeDMLConvention,
			Level:      driverV2.RuleLevelNotice,
			Params: []*SourceParam{
				{Key: "max_table_connect_number", Value: "3", Desc: plocale.PgRule041Params1, Type: params.ParamTypeInt},
			},
		},
		Message:              plocale.PgRule041Message,
		AstSQLHandler:        rule41,
		AllowOffline:         true,
		NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId42, Desc: plocale.PgRule042Desc, Annotation: plocale.PgRule042Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelError},
		Message: plocale.PgRule042Message, AstSQLHandler: rule42, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId43, Desc: plocale.PgRule043Desc, Annotation: plocale.PgRule043Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule043Message, AstSQLHandler: rule43, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId44, Desc: plocale.PgRule044Desc, Annotation: plocale.PgRule044Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule044Message, AstSQLHandler: rule44, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule:    SourceRule{Name: RuleId45, Desc: plocale.PgRule045Desc, Annotation: plocale.PgRule045Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule045Message, RawSQLHandler: rule45, AllowOffline: false, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId47, Desc: plocale.PgRule047Desc, Annotation: plocale.PgRule047Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule047Message, AstSQLHandler: rule47, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId48, Desc: plocale.PgRule048Desc, Annotation: plocale.PgRule048Annotation, Category: plocale.PgRuleTypeDDLConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "expect_created_time", Value: "created_time", Desc: plocale.PgRule048Params1, Type: params.ParamTypeString}},
		},
		Message: plocale.PgRule048Message, AstSQLHandler: rule48, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId49, Desc: plocale.PgRule049Desc, Annotation: plocale.PgRule049Annotation, Category: plocale.PgRuleTypeDDLConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "expect_modified_time", Value: "modified_time", Desc: plocale.PgRule049Params1, Type: params.ParamTypeString}},
		},
		Message: plocale.PgRule049Message, AstSQLHandler: rule49, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId50, Desc: plocale.PgRule050Desc, Annotation: plocale.PgRule050Annotation, Category: plocale.PgRuleTypeNamingConvention, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "expect_object_name_character_max_length", Value: "63", Desc: plocale.PgRule050Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule050Message, AstSQLHandler: rule50, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule:    SourceRule{Name: RuleId51, Desc: plocale.PgRule051Desc, Annotation: plocale.PgRule051Annotation, Category: plocale.PgRuleTypeDDLConvention, Level: driverV2.RuleLevelError},
		Message: plocale.PgRule051Message, AstSQLHandler: rule51, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId52, Desc: plocale.PgRule052Desc, Annotation: plocale.PgRule052Annotation, Category: plocale.PgRuleTypeIndexOptimization, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "expect_index_min_cell_division_percentage", Value: "70", Desc: plocale.PgRule052Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule052Message, AstSQLHandler: rule52, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId53, Desc: plocale.PgRule053Desc, Annotation: plocale.PgRule053Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule053Message, AstSQLHandler: rule53, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId54, Desc: plocale.PgRule054Desc, Annotation: plocale.PgRule054Annotation, Category: plocale.PgRuleTypeNamingConvention, Level: driverV2.RuleLevelError},
		Message: plocale.PgRule054Message, AstSQLHandler: rule54, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId55, Desc: plocale.PgRule055Desc, Annotation: plocale.PgRule055Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "expect_bind_variable_max_number", Value: "100", Desc: plocale.PgRule055Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule055Message, AstSQLHandler: rule55, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId56, Desc: plocale.PgRule056Desc, Annotation: plocale.PgRule056Annotation, Category: plocale.PgRuleTypeNamingConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "expect_fixed_prefix", Value: "tmp_", Desc: plocale.PgRule056Params1, Type: params.ParamTypeString}},
		},
		Message: plocale.PgRule056Message, AstSQLHandler: rule56, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule:    SourceRule{Name: RuleId57, Desc: plocale.PgRule057Desc, Annotation: plocale.PgRule057Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule057Message, RawSQLHandler: rule57, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule:    SourceRule{Name: RuleId59, Desc: plocale.PgRule059Desc, Annotation: plocale.PgRule059Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule059Message, AstSQLHandler: rule59, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId60, Desc: plocale.PgRule060Desc, Annotation: plocale.PgRule060Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelError},
		Message: plocale.PgRule060Message, AstSQLHandler: rule60, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId61, Desc: plocale.PgRule061Desc, Annotation: plocale.PgRule061Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelError},
		Message: plocale.PgRule061Message, AstSQLHandler: rule61, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId62, Desc: plocale.PgRule062Desc, Annotation: plocale.PgRule062Annotation, Category: plocale.PgRuleTypeIndexConvention, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "expect_max_index_number", Value: "5", Desc: plocale.PgRule062Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule062Message, AstSQLHandler: rule62, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId63, Desc: plocale.PgRule063Desc, Annotation: plocale.PgRule063Annotation, Category: plocale.PgRuleTypeIndexConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule063Message, AstSQLHandler: rule63, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId64, Desc: plocale.PgRule064Desc, Annotation: plocale.PgRule064Annotation, Category: plocale.PgRuleTypeIndexConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "max_index_count", Value: "2", Desc: plocale.PgRule064Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule064Message, AstSQLHandler: rule64, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId65, Desc: plocale.PgRule065Desc, Annotation: plocale.PgRule065Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule065Message, AstSQLHandler: rule65, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId66, Desc: plocale.PgRule066Desc, Annotation: plocale.PgRule066Annotation, Category: plocale.PgRuleTypeNamingConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule066Message, RawSQLHandler: rule66, AllowOffline: true,
	},
	{
		Rule: SourceRule{Name: RuleId67, Desc: plocale.PgRule067Desc, Annotation: plocale.PgRule067Annotation, Category: plocale.PgRuleTypeNamingConvention, Level: driverV2.RuleLevelError,
			Params: []*SourceParam{{Key: "expect_fixed_prefix", Value: "uniq_", Desc: plocale.PgRule067Params1, Type: params.ParamTypeString}},
		},
		Message: plocale.PgRule067Message, AstSQLHandler: rule67, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId68, Desc: plocale.PgRule068Desc, Annotation: plocale.PgRule068Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "max_insert_rows", Value: "100", Desc: plocale.PgRule068Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule068Message, AstSQLHandler: rule68, AllowOffline: true, NotAllowOfflineStmts: nil,
	},
	{
		Rule:    SourceRule{Name: RuleId69, Desc: plocale.PgRule069Desc, Annotation: plocale.PgRule069Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule069Message, RawSQLHandler: rule69, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId70, Desc: plocale.PgRule070Desc, Annotation: plocale.PgRule070Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "max_in_parameters_number", Value: "50", Desc: plocale.PgRule070Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule070Message, AstSQLHandler: rule70, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId71, Desc: plocale.PgRule071Desc, Annotation: plocale.PgRule071Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule071Message, AstSQLHandler: rule71, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId72, Desc: plocale.PgRule072Desc, Annotation: plocale.PgRule072Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "specified_functions_set", Value: "sha1,sha256,sqrt,md5", Desc: plocale.PgRule072Params1, Type: params.ParamTypeString}},
		},
		Message: plocale.PgRule072Message, AstSQLHandler: rule72, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId73, Desc: plocale.PgRule073Desc, Annotation: plocale.PgRule073Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice,
			Params: []*SourceParam{{Key: "max_join_number", Value: "3", Desc: plocale.PgRule073Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule073Message, AstSQLHandler: rule73, AllowOffline: false,
	},
	{
		Rule: SourceRule{Name: RuleId74, Desc: plocale.PgRule074Desc, Annotation: plocale.PgRule074Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelError,
			Params: []*SourceParam{{Key: "expect_long_field_length", Value: "2000", Desc: plocale.PgRule074Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule074Message, AstSQLHandler: rule74, AllowOffline: false, NotAllowOfflineStmts: nil,
	},
	{
		Rule: SourceRule{Name: RuleId75, Desc: plocale.PgRule075Desc, Annotation: plocale.PgRule075Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "max_rows", Value: "1000", Desc: plocale.PgRule075Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule075Message, AstSQLHandler: rule75, AllowOffline: true,
	},
	{
		Rule:    SourceRule{Name: RuleId76, Desc: plocale.PgRule076Desc, Annotation: plocale.PgRule076Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule076Message, AstSQLHandler: rule76, AllowOffline: true,
	},
	{
		Rule: SourceRule{Name: RuleId77, Desc: plocale.PgRule077Desc, Annotation: plocale.PgRule077Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelWarn,
			Params: []*SourceParam{{Key: "expected_subQuery_nesting_layers", Value: "3", Desc: plocale.PgRule077Params1, Type: params.ParamTypeInt}},
		},
		Message: plocale.PgRule077Message, AstSQLHandler: rule77, AllowOffline: true,
	},
	{
		Rule:    SourceRule{Name: RuleId78, Desc: plocale.PgRule078Desc, Annotation: plocale.PgRule078Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule078Message, AstSQLHandler: rule78, AllowOffline: false,
	},
	{
		Rule:    SourceRule{Name: RuleId79, Desc: plocale.PgRule079Desc, Annotation: plocale.PgRule079Annotation, Category: plocale.PgRuleTypeDMLConvention, Level: driverV2.RuleLevelNotice},
		Message: plocale.PgRule079Message, AstSQLHandler: rule79, AllowOffline: true,
	},
	{
		Rule:    SourceRule{Name: RuleId80, Desc: plocale.PgRule080Desc, Annotation: plocale.PgRule080Annotation, Category: plocale.PgRuleTypeSuggestion, Level: driverV2.RuleLevelWarn},
		Message: plocale.PgRule080Message, AstSQLHandler: rule80, AllowOffline: false, NotAllowOfflineStmts: nil,
	},
}

func init() {
	RuleHandlers = GenerateI18nRuleHandlers(plocale.Bundle, sourceRuleHandlers, dbTypePG)
	for _, rh := range RuleHandlers {
		RuleHandlerMap[rh.Rule.Name] = rh
	}
}
