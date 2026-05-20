package parser

import (
	parser "actiontech.cloud/sqle/pg_query_go/v5"
)

func Fingerprint(oneSql string) (fingerprint string, err error) {
	fingerprint, err = parser.Normalize(oneSql)
	if err != nil {
		return "", err
	}
	return
}

// ParseSQL return type []*parser.RawStmt
func ParseSQL(sql string) ([]*parser.RawStmt, error) {
	result, err := parser.Parse(sql)
	if err != nil {
		return nil, err
	}
	stmts := make([]*parser.RawStmt, 0, len(result.Stmts))
	for _, stmt := range result.Stmts {
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}
