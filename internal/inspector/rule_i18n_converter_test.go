package inspector

import (
	"testing"

	"github.com/actiontech/dms/pkg/dms-common/i18nPkg"
	"github.com/actiontech/sqle-pg-plugin/plocale"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestLocalizeMessage_NilMessage(t *testing.T) {
	result := localizeMessage(nil)
	if result != nil {
		t.Errorf("expected nil for nil message, got %v", result)
	}
}

func TestLocalizeMessage_WithoutArgs(t *testing.T) {
	msg := &i18n.Message{ID: "PgRule001Desc", Other: "不建议使用select *"}
	result := localizeMessage(msg)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	zhStr := result.GetStrInLang(language.Chinese)
	if zhStr == "" {
		t.Error("expected non-empty Chinese string")
	}
	enStr := result.GetStrInLang(language.English)
	if enStr == "" {
		t.Error("expected non-empty English string")
	}
	if zhStr == enStr {
		t.Errorf("Chinese and English should differ, both are: %s", zhStr)
	}
}

func TestLocalizeMessage_WithArgs(t *testing.T) {
	msg := &i18n.Message{ID: "PgRule009Message", Other: "表字段不建议超过%d个，目前有%d个"}
	result := localizeMessage(msg, 50, 60)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	zhStr := result.GetStrInLang(language.Chinese)
	if zhStr == "" {
		t.Error("expected non-empty Chinese string")
	}
	// The Chinese string should contain the formatted numbers
	if !containsStr(zhStr, "50") || !containsStr(zhStr, "60") {
		t.Errorf("expected Chinese string to contain '50' and '60', got: %s", zhStr)
	}
}

func TestGenerateI18nRuleHandlers(t *testing.T) {
	testCases := []struct {
		name           string
		sourceHandlers []SourceHandler
		expectCount    int
	}{
		{
			name: "basic rule without params",
			sourceHandlers: []SourceHandler{
				{
					Rule: SourceRule{
						Name:       "test_rule_1",
						Desc:       plocale.PgRule001Desc,
						Annotation: plocale.PgRule001Annotation,
						Category:   plocale.PgRuleTypeDQLConvention,
						Level:      driverV2.RuleLevelError,
					},
					Message:      plocale.PgRule001Message,
					AllowOffline: true,
				},
			},
			expectCount: 1,
		},
		{
			name: "rule with params",
			sourceHandlers: []SourceHandler{
				{
					Rule: SourceRule{
						Name:       "test_rule_2",
						Desc:       plocale.PgRule009Desc,
						Annotation: plocale.PgRule009Annotation,
						Category:   plocale.PgRuleTypeDDLConvention,
						Level:      driverV2.RuleLevelWarn,
						Params: []*SourceParam{
							{Key: "max_column_count", Value: "50", Desc: plocale.PgRule009Params1, Type: params.ParamTypeInt},
						},
					},
					Message:      plocale.PgRule009Message,
					AllowOffline: true,
				},
			},
			expectCount: 1,
		},
		{
			name: "multiple rules",
			sourceHandlers: []SourceHandler{
				{
					Rule: SourceRule{
						Name:       "test_rule_a",
						Desc:       plocale.PgRule001Desc,
						Annotation: plocale.PgRule001Annotation,
						Category:   plocale.PgRuleTypeDQLConvention,
						Level:      driverV2.RuleLevelError,
					},
					Message:      plocale.PgRule001Message,
					AllowOffline: true,
				},
				{
					Rule: SourceRule{
						Name:       "test_rule_b",
						Desc:       plocale.PgRule002Desc,
						Annotation: plocale.PgRule002Annotation,
						Category:   plocale.PgRuleTypeDQLConvention,
						Level:      driverV2.RuleLevelError,
					},
					Message:      plocale.PgRule002Message,
					AllowOffline: true,
				},
			},
			expectCount: 2,
		},
		{
			name:           "empty source handlers",
			sourceHandlers: []SourceHandler{},
			expectCount:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rhs := GenerateI18nRuleHandlers(plocale.Bundle, tc.sourceHandlers, "PostgreSQL")
			if len(rhs) != tc.expectCount {
				t.Fatalf("expected %d rule handlers, got %d", tc.expectCount, len(rhs))
			}
			for i, rh := range rhs {
				if rh.Rule.Name != tc.sourceHandlers[i].Rule.Name {
					t.Errorf("rule[%d]: expected name %q, got %q", i, tc.sourceHandlers[i].Rule.Name, rh.Rule.Name)
				}
				if rh.Rule.Level != tc.sourceHandlers[i].Rule.Level {
					t.Errorf("rule[%d]: expected level %v, got %v", i, tc.sourceHandlers[i].Rule.Level, rh.Rule.Level)
				}
				if rh.AllowOffline != tc.sourceHandlers[i].AllowOffline {
					t.Errorf("rule[%d]: expected AllowOffline %v, got %v", i, tc.sourceHandlers[i].AllowOffline, rh.AllowOffline)
				}
				if rh.Message != tc.sourceHandlers[i].Message {
					t.Errorf("rule[%d]: Message pointer should be preserved", i)
				}
				// Verify I18nRuleInfo is populated
				if rh.Rule.I18nRuleInfo == nil {
					t.Errorf("rule[%d]: I18nRuleInfo should not be nil", i)
				}
				// Verify params count matches
				if len(rh.Rule.Params) != len(tc.sourceHandlers[i].Rule.Params) {
					t.Errorf("rule[%d]: expected %d params, got %d", i, len(tc.sourceHandlers[i].Rule.Params), len(rh.Rule.Params))
				}
			}
		})
	}
}

func TestConvertSourceRule(t *testing.T) {
	testCases := []struct {
		name       string
		sourceRule SourceRule
		dbType     string
	}{
		{
			name: "rule without params",
			sourceRule: SourceRule{
				Name:       "test_convert_1",
				Desc:       plocale.PgRule001Desc,
				Annotation: plocale.PgRule001Annotation,
				Category:   plocale.PgRuleTypeDQLConvention,
				Level:      driverV2.RuleLevelError,
			},
			dbType: "PostgreSQL",
		},
		{
			name: "rule with params",
			sourceRule: SourceRule{
				Name:       "test_convert_2",
				Desc:       plocale.PgRule012Desc,
				Annotation: plocale.PgRule012Annotation,
				Category:   plocale.PgRuleTypeNamingConvention,
				Level:      driverV2.RuleLevelError,
				Params: []*SourceParam{
					{Key: "expect_index_name_prefix", Value: "idx_", Desc: plocale.PgRule012Params1, Type: params.ParamTypeString},
				},
			},
			dbType: "PostgreSQL",
		},
		{
			name: "rule with multiple params",
			sourceRule: SourceRule{
				Name:       "test_convert_3",
				Desc:       plocale.PgRule009Desc,
				Annotation: plocale.PgRule009Annotation,
				Category:   plocale.PgRuleTypeDDLConvention,
				Level:      driverV2.RuleLevelWarn,
				Params: []*SourceParam{
					{Key: "param1", Value: "val1", Desc: plocale.PgRule009Params1, Type: params.ParamTypeInt},
					{Key: "param2", Value: "val2", Desc: plocale.PgRule010Params1, Type: params.ParamTypeString},
				},
			},
			dbType: "PostgreSQL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := ConvertSourceRule(plocale.Bundle, &tc.sourceRule, tc.dbType)
			if rule == nil {
				t.Fatal("expected non-nil rule")
			}
			if rule.Name != tc.sourceRule.Name {
				t.Errorf("expected name %q, got %q", tc.sourceRule.Name, rule.Name)
			}
			if rule.Level != tc.sourceRule.Level {
				t.Errorf("expected level %v, got %v", tc.sourceRule.Level, rule.Level)
			}
			// Verify I18nRuleInfo is populated for all language tags
			if rule.I18nRuleInfo == nil {
				t.Fatal("I18nRuleInfo should not be nil")
			}
			for _, tag := range plocale.Bundle.LanguageTags() {
				info, ok := rule.I18nRuleInfo[tag]
				if !ok {
					t.Errorf("missing I18nRuleInfo for language tag %v", tag)
					continue
				}
				if info.Desc == "" {
					t.Errorf("Desc should not be empty for language tag %v", tag)
				}
				if info.Annotation == "" {
					t.Errorf("Annotation should not be empty for language tag %v", tag)
				}
				if info.Category == "" {
					t.Errorf("Category should not be empty for language tag %v", tag)
				}
			}
			// Verify params
			if len(rule.Params) != len(tc.sourceRule.Params) {
				t.Fatalf("expected %d params, got %d", len(tc.sourceRule.Params), len(rule.Params))
			}
			for i, sp := range tc.sourceRule.Params {
				p := rule.Params[i]
				if p.Key != sp.Key {
					t.Errorf("param[%d]: expected key %q, got %q", i, sp.Key, p.Key)
				}
				if p.Value != sp.Value {
					t.Errorf("param[%d]: expected value %q, got %q", i, sp.Value, p.Value)
				}
				if p.Type != sp.Type {
					t.Errorf("param[%d]: expected type %v, got %v", i, sp.Type, p.Type)
				}
				if p.Desc == "" {
					t.Errorf("param[%d]: Desc should not be empty", i)
				}
				if p.I18nDesc == nil {
					t.Errorf("param[%d]: I18nDesc should not be nil", i)
				}
			}
		})
	}
}

func TestGenAllI18nRuleInfo(t *testing.T) {
	sr := &SourceRule{
		Name:       "test_gen_info",
		Desc:       plocale.PgRule001Desc,
		Annotation: plocale.PgRule001Annotation,
		Category:   plocale.PgRuleTypeDQLConvention,
		Level:      driverV2.RuleLevelError,
	}

	result := genAllI18nRuleInfo(plocale.Bundle, sr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	tags := plocale.Bundle.LanguageTags()
	if len(result) != len(tags) {
		t.Errorf("expected %d language entries, got %d", len(tags), len(result))
	}

	// Verify Chinese and English have different values
	zhInfo := result[language.Chinese]
	enInfo := result[language.English]
	if zhInfo == nil || enInfo == nil {
		t.Fatal("expected both Chinese and English entries")
	}
	if zhInfo.Desc == enInfo.Desc {
		t.Errorf("Chinese and English Desc should differ: zh=%q, en=%q", zhInfo.Desc, enInfo.Desc)
	}
	if zhInfo.Annotation == enInfo.Annotation {
		t.Errorf("Chinese and English Annotation should differ: zh=%q, en=%q", zhInfo.Annotation, enInfo.Annotation)
	}
}

func TestGenerateI18nRuleHandlers_PreservesHandlerFunctions(t *testing.T) {
	handlerCalled := false
	testHandler := func(ctx interface{}, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (i18nPkg.I18nStr, error) {
		handlerCalled = true
		return nil, nil
	}
	_ = handlerCalled
	_ = testHandler

	// Verify that AstSQLHandler and RawSQLHandler function pointers are preserved
	sh := SourceHandler{
		Rule: SourceRule{
			Name:       "test_preserve_handler",
			Desc:       plocale.PgRule001Desc,
			Annotation: plocale.PgRule001Annotation,
			Category:   plocale.PgRuleTypeDQLConvention,
			Level:      driverV2.RuleLevelError,
		},
		Message:       plocale.PgRule001Message,
		AstSQLHandler: rule1,
		AllowOffline:  true,
	}

	rhs := GenerateI18nRuleHandlers(plocale.Bundle, []SourceHandler{sh}, "PostgreSQL")
	if len(rhs) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(rhs))
	}
	if rhs[0].AstSQLHandler == nil {
		t.Error("AstSQLHandler should not be nil")
	}
	if rhs[0].RawSQLHandler != nil {
		t.Error("RawSQLHandler should be nil when not set")
	}
}

// TestRuleHandlersInit verifies that the init() function correctly populates
// RuleHandlers and RuleHandlerMap from sourceRuleHandlers.
func TestRuleHandlersInit(t *testing.T) {
	if len(RuleHandlers) == 0 {
		t.Fatal("RuleHandlers should be populated by init()")
	}
	if len(RuleHandlerMap) == 0 {
		t.Fatal("RuleHandlerMap should be populated by init()")
	}
	if len(RuleHandlers) != len(RuleHandlerMap) {
		t.Errorf("RuleHandlers (%d) and RuleHandlerMap (%d) should have the same length",
			len(RuleHandlers), len(RuleHandlerMap))
	}
	// Verify each handler in the slice is also in the map
	for _, rh := range RuleHandlers {
		mapped, ok := RuleHandlerMap[rh.Rule.Name]
		if !ok {
			t.Errorf("rule %q from RuleHandlers not found in RuleHandlerMap", rh.Rule.Name)
			continue
		}
		if mapped.Rule.Name != rh.Rule.Name {
			t.Errorf("RuleHandlerMap[%q] has name %q", rh.Rule.Name, mapped.Rule.Name)
		}
		if mapped.Rule.I18nRuleInfo == nil {
			t.Errorf("rule %q should have I18nRuleInfo", rh.Rule.Name)
		}
		// Some rules (e.g. global config rules like pg_024, pg_025) may intentionally have no Message
		_ = mapped.Message
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
