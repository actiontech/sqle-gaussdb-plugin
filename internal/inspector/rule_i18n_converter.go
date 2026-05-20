package inspector

import (
	"context"

	"github.com/actiontech/dms/pkg/dms-common/i18nPkg"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// SourceParam defines a rule parameter with i18n support for its description.
type SourceParam struct {
	Key   string
	Value string
	Desc  *i18n.Message
	Type  params.ParamType
}

// SourceRule defines a rule with i18n support for its metadata fields.
type SourceRule struct {
	Name       string
	Desc       *i18n.Message
	Annotation *i18n.Message
	Category   *i18n.Message
	Level      driverV2.RuleLevel
	Params     []*SourceParam
}

// SourceHandler combines a SourceRule with its i18n Message and handler functions.
type SourceHandler struct {
	Rule                 SourceRule
	Message              *i18n.Message
	AstSQLHandler        func(ctx context.Context, rule *driverV2.Rule, astSQL interface{}, nextSQL []string) (i18nPkg.I18nStr, error)
	RawSQLHandler        func(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (i18nPkg.I18nStr, error)
	AllowOffline         bool
	NotAllowOfflineStmts []interface{}
}

// GenerateI18nRuleHandlers converts a slice of SourceHandler into a slice of RuleHandler
// by resolving all i18n Messages through the provided Bundle.
func GenerateI18nRuleHandlers(bundle *i18nPkg.Bundle, shs []SourceHandler, dbType string) []RuleHandler {
	rhs := make([]RuleHandler, len(shs))
	for k, v := range shs {
		rhs[k] = RuleHandler{
			Rule:                 *ConvertSourceRule(bundle, &v.Rule, dbType),
			Message:              v.Message,
			AstSQLHandler:        v.AstSQLHandler,
			RawSQLHandler:        v.RawSQLHandler,
			AllowOffline:         v.AllowOffline,
			NotAllowOfflineStmts: v.NotAllowOfflineStmts,
		}
	}
	return rhs
}

// ConvertSourceRule converts a SourceRule into a driverV2.Rule by resolving
// all i18n Messages into their localized forms.
func ConvertSourceRule(bundle *i18nPkg.Bundle, sr *SourceRule, dbType string) *driverV2.Rule {
	r := &driverV2.Rule{
		Name:         sr.Name,
		Level:        sr.Level,
		Params:       make(params.Params, 0, len(sr.Params)),
		I18nRuleInfo: genAllI18nRuleInfo(bundle, sr),
	}
	for _, v := range sr.Params {
		r.Params = append(r.Params, &params.Param{
			Key:      v.Key,
			Value:    v.Value,
			Desc:     bundle.LocalizeMsgByLang(i18nPkg.DefaultLang, v.Desc),
			I18nDesc: bundle.LocalizeAll(v.Desc),
			Type:     v.Type,
		})
	}
	return r
}

// genAllI18nRuleInfo generates a map of language.Tag to RuleInfo for all
// languages supported by the Bundle.
func genAllI18nRuleInfo(bundle *i18nPkg.Bundle, sr *SourceRule) map[language.Tag]*driverV2.RuleInfo {
	result := make(map[language.Tag]*driverV2.RuleInfo, len(bundle.LanguageTags()))
	for _, langTag := range bundle.LanguageTags() {
		result[langTag] = &driverV2.RuleInfo{
			Desc:       bundle.LocalizeMsgByLang(langTag, sr.Desc),
			Annotation: bundle.LocalizeMsgByLang(langTag, sr.Annotation),
			Category:   bundle.LocalizeMsgByLang(langTag, sr.Category),
		}
	}
	return result
}
