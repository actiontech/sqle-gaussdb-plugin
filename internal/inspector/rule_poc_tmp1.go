package inspector

import (
	"context"
	"fmt"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/pkg/errors"
)

const (
	SQLEPOC01 = "SQLEPOC01"
)

// todo POC USE , later split
func init() {
	rh := RuleHandler{
		Rule: driverV2.Rule{
			Name:       SQLEPOC01,
			Desc:       "SQL查询条件必须能走索引，避免扫描行数大于阈值",
			Annotation: "SQL查询条件必须能走索引，避免扫描行数大于阈值,可以提高查询性能,节省系统资源,增强数据库并发处理能力",
			Level:      driverV2.RuleLevelError,
			Category:   RuleTypeDMLConvention,
			Params: params.Params{&params.Param{
				Key:   DefaultSingleParamKeyName,
				Value: "10000",
				Desc:  "扫描行数上限",
				Type:  params.ParamTypeInt,
			},
			},
		},
		Message:              "SQL查询条件必须能走索引，避免扫描行数大于%d",
		RawSQLHandler:        RuleSQLEPOC01,
		AllowOffline:         false,
		NotAllowOfflineStmts: nil,
	}
	RuleHandlers = append(RuleHandlers, rh)
	RuleHandlerMap[rh.Rule.Name] = rh
}

func RuleSQLEPOC01(ctx context.Context, rule *driverV2.Rule, sql string, nextSQL []string) (string, error) {
	node, err := sqlParserFuncV2(sql)
	if err != nil {
		return "", errors.Wrap(err, "parse sql")
	}

	maxScanRowCount := utilGetRuleParamInt(rule, DefaultSingleParamKeyName)
	if maxScanRowCount <= 0 {
		return "", errors.New("invalid maxScanRowCount")
	}

	checkUsedIndexAndScanRow := func(plan *PlanType, maxScanRowCount int) bool {
		isOk := false
		utilVisitPlan(plan, func(planNode *PlanType) bool {
			isIndexScan := utilIsIndexScanType(ScanType(planNode.NodeType))
			if isIndexScan && planNode.PlanRows <= int64(maxScanRowCount) {
				isOk = true
			}
			return false
		})
		return !isOk
	}

	switch node.GetStmt().GetNode().(type) {
	case *parser.Node_SelectStmt, *parser.Node_UpdateStmt, *parser.Node_DeleteStmt:
		plan, err := utilGetExecutionPlan(ctx, sql)
		if err != nil {
			return "", err
		}

		if checkUsedIndexAndScanRow(plan, maxScanRowCount) {
			return fmt.Sprintf(RuleHandlerMap[rule.Name].Message, maxScanRowCount), nil
		}
	}

	return "", nil
}
