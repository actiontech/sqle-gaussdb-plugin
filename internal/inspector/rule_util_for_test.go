package inspector

import (
	"context"
	"testing"

	parser "actiontech.cloud/sqle/pg_query_go/v5"
	"github.com/actiontech/sqle-pg-plugin/internal/executor"
	driverV2 "github.com/actiontech/sqle/sqle/driver/v2"
	driverPkg "github.com/actiontech/sqle/sqle/pkg/driver"
	"github.com/actiontech/sqle/sqle/pkg/params"
	"github.com/stretchr/testify/assert"
)

func MockPGContext2(e *executor.Executor) *PgContext {
	return &PgContext{
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
		Executor:             e,
		UsingType:            UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			CurrentSchema: "exist_db",
			SchemaInfoMap: map[string]*SchemaInfo{
				"exist_db": {
					TableInfoList: []*TableInfo{
						{
							TableName:    "exist_tb_1",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "nextval('exist_db.exist_tb_1_id_seq'::regclass)",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_1_pkey",
									ConstraintName:  "exist_tb_1_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "'v1'::character varying",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "idx_1",
									ConstraintName:  "idx_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    10485760,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
							ConstraintList: []*ConstraintInfo{
								{
									ConstraintName: "exist_tb_1_pkey",
									ConstraintType: ConstraintTypeP,
									OwnerName:      "exist_db",
									ColumnList:     []string{"id"},
								},
							},
						},
						{
							TableName:    "exist_tb_2",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "nextval('exist_db.exist_tb_2_id_seq'::regclass)",
									IsPrimaryColumn: true,
									IsUnique:        true,
									IndexName:       "exist_tb_2_id_key",
									ConstraintName:  "exist_tb_2_id_key",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "user_id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "pk_test_1",
									IsLoadFromDb:    true,
								},
							},
						},
						{
							TableName:              "exist_tb_3",
							OwnerName:              "exist_db",
							IsLoadFromDb:           true,
							DistributionColumnName: "v3", // 分片键
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "GENERATED BY DEFAULT AS IDENTITY",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_3_pkey",
									ConstraintName:  "exist_tb_3_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v3",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
						},
						{
							TableName:    "exist_tb_9",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "bigserial",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_9_pkey",
									ConstraintName:  "exist_tb_9_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "uniq_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v3",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "uniq_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v4",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "v5",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
						},
					},
					IndexInfoList: []*IndexInfo{
						{
							IndexName:    "exist_tb_1_uniq_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"v1", "v2"}, // 索引包含列 v1 和 v2
							IsUnique:     true,                 // 是否唯一索引
							IsLoadFromDb: true,                 // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_1_pkey",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_1_idx_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"v1"}, // 列名列表
							IsUnique:     false,          // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_2_id_key",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_2",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_2_user_id_key",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_2",
							ColumnList:   []string{"user_id"}, // 列名列表
							IsUnique:     false,               // 是否唯一索引
							IsLoadFromDb: true,                // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_3_pkey",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_3",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						/*
							CREATE TABLE exist_db.exist_tb_9 (
								id bigserial PRIMARY KEY,
								v1 integer,
								v2 varchar(255),
								v3 integer,
								v4 integer,
								v5 integer
							);

							-- 添加唯一性约束
							ALTER TABLE exist_db.exist_tb_9 ADD CONSTRAINT uniq_1 UNIQUE (v2, v3);

							-- 创建索引
							CREATE INDEX exist_tb_9_idx_1 ON exist_db.exist_tb_9 (v1, v2, v3, v4);
							CREATE INDEX exist_tb_9_idx_2 ON exist_db.exist_tb_9 (v3);

							-- 添加注释
							COMMENT ON TABLE exist_db.exist_tb_9 IS 'unit test';
							COMMENT ON COLUMN exist_db.exist_tb_9.id IS 'unit test';
						*/
						{
							IndexName:    "exist_tb_9_idx_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_9",
							ColumnList:   []string{"v1", "v2", "v3", "v4"}, // 列名列表
							IsUnique:     false,                            // 是否唯一索引
							IsLoadFromDb: true,                             // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_9_idx_2",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_9",
							ColumnList:   []string{"v3"}, // 列名列表
							IsUnique:     false,          // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName: "exist_tb_9_idx_func",
							OwnerName: "exist_db",
							TableName: "exist_tb_9",
							FuncCallList: []*parser.FuncCall{ // 函数列表
								{
									Funcname: []*parser.Node{&parser.Node{Node: &parser.Node_String_{String_: &parser.String{Sval: "lower"}}}},
									Args:     []*parser.Node{&parser.Node{Node: &parser.Node_ColumnRef{ColumnRef: &parser.ColumnRef{Fields: []*parser.Node{{Node: &parser.Node_String_{String_: &parser.String{Sval: "v5"}}}}}}}},
								},
							},
							IsUnique:     false, // 是否唯一索引
							IsLoadFromDb: true,  // 是否从数据库装载
						},
					},
				},
			},
		},
	}
}

// ====  Mock Db info start ====
func MockPGContext(e *executor.Executor) *PgContext {
	return &PgContext{
		ExecutionPlanCache:   make(map[string]*[]PlanType),
		DeletedSchemaMap:     make(map[string]string),
		DeletedTableMap:      make(map[string]string),
		DeletedIndexMap:      make(map[string]string),
		DeletedColumnMap:     make(map[string]string),
		DeletedConstraintMap: make(map[string]string),
		Executor:             e,
		UsingType:            UsingTypeOnline,
		DatabaseInfo: &DatabaseInfo{
			DatabaseDefaultCollate: "en_US.UTF-8",
			CurrentSchema:          "exist_db",
			SchemaInfoMap: map[string]*SchemaInfo{
				"exist_db": {
					TableInfoList: []*TableInfo{
						{
							TableName:    "exist_view_1",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							IsView:       true,
						},
						{
							TableName:              "exist_tb_1",
							OwnerName:              "exist_db",
							IsLoadFromDb:           true,
							DistributionColumnName: "id_shd_key", // 分片键
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id_shd_key",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "nextval('exist_db.exist_tb_1_id_seq'::regclass)",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_1_pkey",
									ConstraintName:  "exist_tb_1_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c1_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "'c1_with_idx'::character varying",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_1_idx_1",
									ConstraintName:  "exist_tb_1_idx_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c2_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_1",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    10485760,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
							ConstraintList: []*ConstraintInfo{
								{
									ConstraintName: "exist_tb_1_pkey",
									ConstraintType: ConstraintTypeP,
									OwnerName:      "exist_db",
									ColumnList:     []string{"id_shd_key"},
								},
							},
						},
						{
							TableName:    "exist_tb_2",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "nextval('exist_db.exist_tb_2_id_seq'::regclass)",
									IsPrimaryColumn: true,
									IsUnique:        true,
									IndexName:       "exist_tb_2_id_key",
									ConstraintName:  "exist_tb_2_id_key",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "user_id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_2",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "pk_test_1",
									IsLoadFromDb:    true,
								},
							},
						},
						{
							TableName:              "exist_tb_3",
							OwnerName:              "exist_db",
							IsLoadFromDb:           true,
							DistributionColumnName: "c3_shd_key", // 分片键
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "GENERATED BY DEFAULT AS IDENTITY",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_3_pkey",
									ConstraintName:  "exist_tb_3_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c1",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c2",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c3_shd_key",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_3",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
						},
						{
							TableName:              "exist_tb_4",
							OwnerName:              "exist_db",
							IsLoadFromDb:           true,
							DistributionColumnName: "id_shd_key", // 分片键
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id_shd_key",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_4",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "nextval('exist_db.exist_tb_4_id_seq'::regclass)",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_4_pkey",
									ConstraintName:  "exist_tb_4_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "date_column",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_4",
									ColumnType:      "date",
									ColumnTypType:   "b",
									ColumnLength:    0,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "CURRENT_DATE",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
						},
						{
							TableName:    "exist_tb_9",
							OwnerName:    "exist_db",
							IsLoadFromDb: true,
							ColumnInfoList: []*ColumnInfo{
								{
									ColumnName:      "id",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "bigint",
									ColumnTypType:   "b",
									ColumnLength:    64,
									ColumnPrecision: 0,
									IsNullable:      false,
									DefaultValue:    "bigserial",
									IsPrimaryColumn: true,
									IsUnique:        false,
									IndexName:       "exist_tb_9_pkey",
									ConstraintName:  "exist_tb_9_pkey",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c1_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c2_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "character varying",
									ColumnTypType:   "b",
									ColumnLength:    255,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "uniq_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c3_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "uniq_1",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c4_with_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "exist_tb_9_idx_1",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
								{
									ColumnName:      "c5_with_func_idx",
									OwnerName:       "exist_db",
									TableName:       "exist_tb_9",
									ColumnType:      "integer",
									ColumnTypType:   "n",
									ColumnLength:    32,
									ColumnPrecision: 0,
									IsNullable:      true,
									DefaultValue:    "",
									IsPrimaryColumn: false,
									IsUnique:        false,
									IndexName:       "",
									ConstraintName:  "",
									IsLoadFromDb:    true,
								},
							},
						},
					},
					IndexInfoList: []*IndexInfo{
						{
							IndexName:    "exist_tb_1_uniq_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"c1_with_idx", "c2_with_idx"},
							IsUnique:     true, // 是否唯一索引
							IsLoadFromDb: true, // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_1_pkey",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsPrimaryKey: true,           // 是否主键
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_1_idx_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_1",
							ColumnList:   []string{"c1_with_idx"},
							IsUnique:     false,
							IsLoadFromDb: true,
						},
						{
							IndexName:    "exist_tb_2_id_key",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_2",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsLoadFromDb: true,           // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_2_user_id_key",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_2",
							ColumnList:   []string{"user_id"}, // 列名列表
							IsUnique:     false,               // 是否唯一索引
							IsLoadFromDb: true,                // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_3_pkey",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_3",
							ColumnList:   []string{"id"}, // 列名列表
							IsUnique:     true,           // 是否唯一索引
							IsPrimaryKey: true,
							IsLoadFromDb: true, // 是否从数据库装载
						},
						/*
							CREATE TABLE exist_db.exist_tb_9 (
								id bigserial PRIMARY KEY,
								v1 integer,
								v2 varchar(255),
								v3 integer,
								v4 integer,
								c5_with_func_idx integer
							);

							-- 添加唯一性约束
							ALTER TABLE exist_db.exist_tb_9 ADD CONSTRAINT uniq_1 UNIQUE (v2, v3);

							-- 创建索引
							CREATE INDEX exist_tb_9_idx_1 ON exist_db.exist_tb_9 (v1, v2, v3, v4);
							CREATE INDEX exist_tb_9_idx_2 ON exist_db.exist_tb_9 (v3);

							-- 添加注释
							COMMENT ON TABLE exist_db.exist_tb_9 IS 'unit test';
							COMMENT ON COLUMN exist_db.exist_tb_9.id IS 'unit test';
						*/
						{
							IndexName:    "exist_tb_9_idx_1",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_9",
							ColumnList:   []string{"c1_with_idx", "c2_with_idx", "c3_with_idx", "c4_with_idx"}, // 列名列表
							IsUnique:     false,                                                                // 是否唯一索引
							IsLoadFromDb: true,                                                                 // 是否从数据库装载
						},
						{
							IndexName:    "exist_tb_9_idx_2",
							OwnerName:    "exist_db",
							TableName:    "exist_tb_9",
							ColumnList:   []string{"c3_with_idx"}, // 列名列表
							IsUnique:     false,                   // 是否唯一索引
							IsLoadFromDb: true,                    // 是否从数据库装载
						},
						{
							IndexName: "exist_tb_9_idx_func",
							OwnerName: "exist_db",
							TableName: "exist_tb_9",
							FuncCallList: []*parser.FuncCall{ // 函数列表
								{
									Funcname: []*parser.Node{&parser.Node{Node: &parser.Node_String_{String_: &parser.String{Sval: "lower"}}}},
									Args:     []*parser.Node{&parser.Node{Node: &parser.Node_ColumnRef{ColumnRef: &parser.ColumnRef{Fields: []*parser.Node{{Node: &parser.Node_String_{String_: &parser.String{Sval: "c5_with_func_idx"}}}}}}}},
								},
							},
							IsUnique:     false, // 是否唯一索引
							IsLoadFromDb: true,  // 是否从数据库装载
						},
					},
				},
			},
		},
	}
}

// ====  Mock Db info end====

func testAuditWithMockExecutor(ruleName string, t *testing.T, sqls []string, expectResult []*testResults, e *executor.Executor) {
	mockPgCtx := MockPGContext2(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext2(ruleName, "", currentSchema, mockPgCtx)
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, sqls)
	if err != nil {
		t.Error(err)
		return
	}
	for i, r := range results {
		if r.Level() != expectResult[i].Level() {
			t.Errorf("expect level is %s, actual is %s\nsqls: %s", expectResult[i].Level(), r.Level(), sqls)
		}
		if r.Message() != expectResult[i].Message() {
			t.Errorf("expect message is %s\n actual is %s\nsqls: %s", expectResult[i].Message(), r.Message(), sqls)
		}
	}
	return
}

func testAuditWithMockPGContext(ruleName string, t *testing.T, sql string, expectResult *testResults) {
	e, _, err := executor.NewMockExecutorForUnitTest()
	if err != nil {
		t.Error(err)
		return
	}

	mockPgCtx := MockPGContext2(e)

	d := newTestDriverWithMockPGContext2(ruleName, "", "", mockPgCtx)
	currentSchema := ""
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	results, err := d.Audit(ctx, []string{sql})
	if err != nil {
		t.Error(err)
		return
	}
	if len(results) != 1 {
		t.Errorf("expect 1 result, actual is %d\nsql: %s", len(results), sql)
		return
	}
	r := results[0]
	if r.Level() != expectResult.Level() {
		t.Errorf("expect level is %s, actual is %s\nsql: %s", expectResult.Level(), r.Level(), sql)
	}
	if r.Message() != expectResult.Message() {
		t.Errorf("expect message is %s\n actual is %s\nsql: %s", expectResult.Message(), r.Message(), sql)
	}
	return
}

func newTestDriverWithMockPGContext2(ruleName, database, schema string, pgCtx *PgContext) *driverImpl {
	d := newTestDriverObs(ruleName, database, schema, pgCtx.Executor)
	d.pgContext = pgCtx
	return d
}

func wrapAuditForUnitTest(ruleName string, e *executor.Executor, sqls []string) ([]*driverV2.AuditResults, error) {
	mockPgCtx := MockPGContext2(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext2(ruleName, "", currentSchema, mockPgCtx)
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)
	return d.Audit(ctx, sqls)
}

func assertRuleResult(t *testing.T, desc, sql string, exps []*testResults, acts []*driverV2.AuditResults) {

	if len(exps) != len(acts) {
		t.Errorf("expect result length(%v) not equal actual result length(%v)", len(exps), len(acts))
		if len(acts) != 0 {
			for i, r := range acts {
				t.Errorf("Get acts[%v]: %v", i, r.Message())
			}
		}
		if len(exps) != 0 {
			for i, r := range exps {
				t.Errorf("Get exps[%v]: %v", i, r.Message())
			}
		}

	}

	if len(exps) == 0 {
		return
	}
	for i, r := range acts {
		if r.Level() != exps[i].Level() || r.Message() != exps[i].Message() {
			t.Errorf("\nEXPECT: [%v]\nACTUAL: [%v]\nSQLS: %s\n", exps[i].Message(), r.Message(), sql)
		}
		// if r.Level() != exps[i].Level() {
		// 	t.Errorf("expect level is [%s], actual is [%s]\nsqls: %s\n", exps[i].Level(), r.Level(), sql)
		// }
		// if r.Message() != exps[i].Message() {
		// 	t.Errorf("expect message is [%s]\n actual is [%s]\nsqls: %s\n", exps[i].Message(), r.Message(), sql)
		// }

	}

}

func setupForRuleGeneral2(t *testing.T, ruleName string, params params.Params) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, _, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// mock 规则审核器
	mockPgCtx := MockPGContext2(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

func setupForRuleGeneral(t *testing.T, ruleName string, params params.Params) (*driverImpl, context.Context) {
	// 创建模拟的数据库执行器
	e, _, err := executor.NewMockExecutorForUnitTest()
	assert.NoError(t, err)

	// mock 规则审核器
	mockPgCtx := MockPGContext(e)
	currentSchema := mockPgCtx.DatabaseInfo.CurrentSchema
	d := newTestDriverWithMockPGContext(ruleName, params, "", currentSchema, mockPgCtx)

	// mock Audit 函数的上下文
	handlerCtx := ruleHandlerContext{CurrentSchema: &currentSchema, Executor: e, PgContext: mockPgCtx}
	ctx := context.WithValue(context.Background(), CtxKeyRuleHandlerCtx, handlerCtx)

	return d, ctx
}

// _DefaultParams 代表没有任何参数的常量定义
var _DefaultParams params.Params = []*params.Param{}
var _RuleAuditPassResults = []*testResults{newTestResults()}

// todo: 删除 newTestDriver 函数
// 原 newTestDriver 不能自定义 params，导致自动化测试不能修改参数阈值
// 为了两个分支代码合并时修改尽量的小，因此暂时先同时保留两个函数
func newTestDriver(ruleName string, params params.Params, database, schema string, conn *executor.Executor) *driverImpl {
	rules := make([]*driverV2.Rule, 0, len(RuleHandlerMap))
	dsnParams := &driverV2.DSN{}
	ah := &driverPkg.AuditHandler{
		SqlParserFn:      SqlParserFunc,
		RuleToRawHandler: make(map[string]driverPkg.RawSQLRuleHandler),
		RuleToASTHandler: make(map[string]driverPkg.AstSQLRuleHandler),
	}
	for _, rh := range RuleHandlers {
		if rh.Rule.Name == RuleId9 {
			rh.Rule.Params.SetParamValue("max_column_count", "10")
		}
		if rh.Rule.Name == ruleName {
			rule := &driverV2.Rule{
				Name:       rh.Rule.Name,
				Desc:       rh.Rule.Desc,
				Annotation: rh.Rule.Annotation,
				Category:   rh.Rule.Category,
				Level:      rh.Rule.Level,
				Knowledge:  rh.Rule.Knowledge,
			}
			if len(params) > 0 {
				rule.Params = params
			} else {
				rule.Params = rh.Rule.Params.Copy()
			}
			rules = append(rules, rule)
			ruleHandler := RuleHandlerMap[ruleName]
			if ruleHandler.RawSQLHandler != nil {
				ah.RuleToRawHandler[ruleName] = ruleHandler.RawSQLHandler
			} else if ruleHandler.AstSQLHandler != nil {
				ah.RuleToASTHandler[ruleName] = ruleHandler.AstSQLHandler
			}
		}
	}
	return NewMockDriver(
		&driverV2.Config{
			DSN:   dsnParams,
			Rules: rules,
		},
		database,
		schema,
		ah,
		conn,
	)
}

func newTestDriverWithMockPGContext(ruleName string, params params.Params, database, schema string, pgCtx *PgContext) *driverImpl {
	d := newTestDriver(ruleName, params, database, schema, pgCtx.Executor)
	d.pgContext = pgCtx
	return d
}
