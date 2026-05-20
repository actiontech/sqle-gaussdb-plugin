package plocale

import "github.com/nicksnyder/go-i18n/v2/i18n"

// Rule Category
var (
	PgRuleTypeDMLConvention     = &i18n.Message{ID: "PgRuleTypeDMLConvention", Other: "DML规范"}
	PgRuleTypeDDLConvention     = &i18n.Message{ID: "PgRuleTypeDDLConvention", Other: "DDL规范"}
	PgRuleTypeDQLConvention     = &i18n.Message{ID: "PgRuleTypeDQLConvention", Other: "DQL规范"}
	PgRuleTypeSuggestion        = &i18n.Message{ID: "PgRuleTypeSuggestion", Other: "使用建议"}
	PgRuleTypeNamingConvention  = &i18n.Message{ID: "PgRuleTypeNamingConvention", Other: "命名规范"}
	PgRuleTypeGlobalConfig      = &i18n.Message{ID: "PgRuleTypeGlobalConfig", Other: "全局配置"}
	PgRuleTypeIndexOptimization = &i18n.Message{ID: "PgRuleTypeIndexOptimization", Other: "索引优化"}
	PgRuleTypeIndexConvention   = &i18n.Message{ID: "PgRuleTypeIndexConvention", Other: "索引规范"}
)

// pg_001
var (
	PgRule001Desc       = &i18n.Message{ID: "PgRule001Desc", Other: "不建议使用select *"}
	PgRule001Annotation = &i18n.Message{ID: "PgRule001Annotation", Other: "select *，影响解析效率，也容易在表结构变化时SQL数据处理未匹配变化，导致与业务预期不符的问题"}
	PgRule001Message    = &i18n.Message{ID: "PgRule001Message", Other: "不建议使用select *"}
)

// pg_002
var (
	PgRule002Desc       = &i18n.Message{ID: "PgRule002Desc", Other: "delete 和 update 语句，必须带where条件"}
	PgRule002Annotation = &i18n.Message{ID: "PgRule002Annotation", Other: "update、delete语句缺少where条件，存在错误更新全表数据的风险"}
	PgRule002Message    = &i18n.Message{ID: "PgRule002Message", Other: "delete 和 update 语句，必须带where条件"}
)

// pg_003
var (
	PgRule003Desc       = &i18n.Message{ID: "PgRule003Desc", Other: "避免使用 having 子句"}
	PgRule003Annotation = &i18n.Message{ID: "PgRule003Annotation", Other: "HAVING 只会在检索出所有记录之后才对结果集进行过滤，这个处理需要排序、统计等操作，使用WHERE子句限制记录的数目，减少这方面的开销"}
	PgRule003Message    = &i18n.Message{ID: "PgRule003Message", Other: "避免使用 having 子句"}
)

// pg_004
var (
	PgRule004Desc       = &i18n.Message{ID: "PgRule004Desc", Other: "禁止除索引外的 drop 操作"}
	PgRule004Annotation = &i18n.Message{ID: "PgRule004Annotation", Other: "建议在执行DROP操作之前，对数据进行备份以及与业务进行操作确认，防止误操作"}
	PgRule004Message    = &i18n.Message{ID: "PgRule004Message", Other: "禁止除索引外的 drop 操作"}
)

// pg_005
var (
	PgRule005Desc       = &i18n.Message{ID: "PgRule005Desc", Other: "禁止使用视图"}
	PgRule005Annotation = &i18n.Message{ID: "PgRule005Annotation", Other: "视图的查询性能较差，同时基表结构变更，都需要对视图进行维护，如果视图可读性差且包含复杂的逻辑，都会增加维护的成本"}
	PgRule005Message    = &i18n.Message{ID: "PgRule005Message", Other: "禁止使用视图"}
)

// pg_006
var (
	PgRule006Desc       = &i18n.Message{ID: "PgRule006Desc", Other: "禁止使用触发器"}
	PgRule006Annotation = &i18n.Message{ID: "PgRule006Annotation", Other: "触发器难以开发和维护，不能高效移植，且在复杂的逻辑以及高并发下，容易出现死锁影响业务"}
	PgRule006Message    = &i18n.Message{ID: "PgRule006Message", Other: "禁止使用触发器"}
)

// pg_007
var (
	PgRule007Desc       = &i18n.Message{ID: "PgRule007Desc", Other: "表建议使用主键"}
	PgRule007Annotation = &i18n.Message{ID: "PgRule007Annotation", Other: "主键有利于后期数据维护，且可提高SQL的执行效率"}
	PgRule007Message    = &i18n.Message{ID: "PgRule007Message", Other: "表建议使用主键"}
)

// pg_008
var (
	PgRule008Desc       = &i18n.Message{ID: "PgRule008Desc", Other: "表不建议使用外键"}
	PgRule008Annotation = &i18n.Message{ID: "PgRule008Annotation", Other: "外键在高并发场景下性能较差，容易造成死锁，同时不利于后期维护（拆分、迁移）"}
	PgRule008Message    = &i18n.Message{ID: "PgRule008Message", Other: "表不建议使用外键"}
)

// pg_009
var (
	PgRule009Desc       = &i18n.Message{ID: "PgRule009Desc", Other: "表字段不建议过多"}
	PgRule009Annotation = &i18n.Message{ID: "PgRule009Annotation", Other: "过度的宽表，会造成数据的大量冗余，后期对性能影响很大"}
	PgRule009Message    = &i18n.Message{ID: "PgRule009Message", Other: "表字段不建议超过%d个，目前有%d个"}
	PgRule009Params1    = &i18n.Message{ID: "PgRule009Params1", Other: "最大字段个数"}
)

// pg_010
var (
	PgRule010Desc       = &i18n.Message{ID: "PgRule010Desc", Other: "单条SQL不建议过长"}
	PgRule010Annotation = &i18n.Message{ID: "PgRule010Annotation", Other: "过长的sql可读性较差，难以维护，且容易引发性能问题"}
	PgRule010Message    = &i18n.Message{ID: "PgRule010Message", Other: "单条SQL不建议过长，长度超过%d"}
	PgRule010Params1    = &i18n.Message{ID: "PgRule010Params1", Other: "SQL长度"}
)

// pg_011
var (
	PgRule011Desc       = &i18n.Message{ID: "PgRule011Desc", Other: "复合索引的列数量不建议超过阈值"}
	PgRule011Annotation = &i18n.Message{ID: "PgRule011Annotation", Other: "复合索引会根据索引列数创建对应组合的索引，列数越多，创建的索引越多，每个索引都会增加磁盘空间的开销，同时增加索引维护的开销，建议不超过3个"}
	PgRule011Message    = &i18n.Message{ID: "PgRule011Message", Other: "复合索引的列数量不建议超过%v个"}
	PgRule011Params1    = &i18n.Message{ID: "PgRule011Params1", Other: "最大索引列数量"}
)

// pg_012
var (
	PgRule012Desc       = &i18n.Message{ID: "PgRule012Desc", Other: "普通索引必须使用固定前缀"}
	PgRule012Annotation = &i18n.Message{ID: "PgRule012Annotation", Other: "统一命名规范，有利于后期维护以及业务开发"}
	PgRule012Message    = &i18n.Message{ID: "PgRule012Message", Other: "普通索引必须要以\"%v\"为前缀"}
	PgRule012Params1    = &i18n.Message{ID: "PgRule012Params1", Other: "索引前缀"}
)

// pg_015
var (
	PgRule015Desc       = &i18n.Message{ID: "PgRule015Desc", Other: "数据库对象命名禁止使用关键字"}
	PgRule015Annotation = &i18n.Message{ID: "PgRule015Annotation", Other: "避免发生冲突，以及混淆"}
	PgRule015Message    = &i18n.Message{ID: "PgRule015Message", Other: "数据库对象命名禁止使用关键字，此次涉及关键字%s"}
)

// pg_016
var (
	PgRule016Desc       = &i18n.Message{ID: "PgRule016Desc", Other: "禁止使用左模糊查询"}
	PgRule016Annotation = &i18n.Message{ID: "PgRule016Annotation", Other: "使用左模糊查询，类似：like '%abc'，会导致SQL无法使用索引"}
	PgRule016Message    = &i18n.Message{ID: "PgRule016Message", Other: "禁止使用左模糊查询，此次涉及的字段%s"}
)

// pg_017
var (
	PgRule017Desc       = &i18n.Message{ID: "PgRule017Desc", Other: "查询的扫描不建议超过指定行数（默认值：10000）"}
	PgRule017Annotation = &i18n.Message{ID: "PgRule017Annotation", Other: "扫描行数越多，SQL的执行效率越低,默认值：10000"}
	PgRule017Message    = &i18n.Message{ID: "PgRule017Message", Other: "存在大表全表扫描，扫描行数不能大于%v"}
	PgRule017Params1    = &i18n.Message{ID: "PgRule017Params1", Other: "最大扫描行数"}
)

// pg_019
var (
	PgRule019Desc       = &i18n.Message{ID: "PgRule019Desc", Other: "From子句中存在多层嵌套"}
	PgRule019Annotation = &i18n.Message{ID: "PgRule019Annotation", Other: "嵌套越深，需要扫描的行数、生成的结果集就越大，SQL的执行效率越低"}
	PgRule019Message    = &i18n.Message{ID: "PgRule019Message", Other: "From子句中存在多层嵌套,超过阈值%d。当前SQL嵌套层数%d"}
	PgRule019Params1    = &i18n.Message{ID: "PgRule019Params1", Other: "最大嵌套层数"}
)

// pg_021
var (
	PgRule021Desc       = &i18n.Message{ID: "PgRule021Desc", Other: "建议避免使用select for update"}
	PgRule021Annotation = &i18n.Message{ID: "PgRule021Annotation", Other: "select for update 会对查询结果集中每行数据都添加排他锁，其他线程对该记录的更新与删除操作都会阻塞，在高并发下，容易造成数据库大量锁等待，影响数据库查询性能"}
	PgRule021Message    = &i18n.Message{ID: "PgRule021Message", Other: "建议避免使用select for update"}
)

// pg_022
var (
	PgRule022Desc       = &i18n.Message{ID: "PgRule022Desc", Other: "建议给表添加注释"}
	PgRule022Annotation = &i18n.Message{ID: "PgRule022Annotation", Other: "表添加注释能够使表的意义更明确，方便日后的维护"}
	PgRule022Message    = &i18n.Message{ID: "PgRule022Message", Other: "建议给这些表添加注释：%v"}
)

// pg_023
var (
	PgRule023Desc       = &i18n.Message{ID: "PgRule023Desc", Other: "建议给列添加注释"}
	PgRule023Annotation = &i18n.Message{ID: "PgRule023Annotation", Other: "列添加注释能够使列的意义更明确，方便日后的维护"}
	PgRule023Message    = &i18n.Message{ID: "PgRule023Message", Other: "建议给表(%v)的这些列添加注释：%v"}
)

// pg_024
var (
	PgRule024Desc       = &i18n.Message{ID: "PgRule024Desc", Other: "在 DML 语句中预计影响行数超过指定值则不回滚"}
	PgRule024Annotation = &i18n.Message{ID: "PgRule024Annotation", Other: "大事务回滚，容易影响数据库性能，使得业务发生波动；具体规则阈值可以根据业务需求调整，默认值：1000"}
	PgRule024Params1    = &i18n.Message{ID: "PgRule024Params1", Other: "回滚语句影响行数"}
)

// pg_025
var (
	PgRule025Desc       = &i18n.Message{ID: "PgRule025Desc", Other: "使用sql语句回滚功能"}
	PgRule025Annotation = &i18n.Message{ID: "PgRule025Annotation", Other: "回滚语句可以挽回错误的sql执行,提供容错机制"}
)

// pg_026
var (
	PgRule026Desc       = &i18n.Message{ID: "PgRule026Desc", Other: "不建议修改列的数据类型"}
	PgRule026Annotation = &i18n.Message{ID: "PgRule026Annotation", Other: "不建议修改列的数据类型"}
	PgRule026Message    = &i18n.Message{ID: "PgRule026Message", Other: "不建议修改列的数据类型"}
)

// pg_027
var (
	PgRule027Desc       = &i18n.Message{ID: "PgRule027Desc", Other: "insert语句未指定列信息"}
	PgRule027Annotation = &i18n.Message{ID: "PgRule027Annotation", Other: "insert 不指定具体列时，如果表上增加了字段可能会导致SQL报错"}
	PgRule027Message    = &i18n.Message{ID: "PgRule027Message", Other: "insert语句未指定列信息"}
)

// pg_028
var (
	PgRule028Desc       = &i18n.Message{ID: "PgRule028Desc", Other: "执行计划存在笛卡尔积，建议检查表关联条件"}
	PgRule028Annotation = &i18n.Message{ID: "PgRule028Annotation", Other: "执行计划显示存在笛卡尔积，说明SQL中可能缺少连接条件，意味着SQL执行效率很低，容易影响数据库性能。"}
	PgRule028Message    = &i18n.Message{ID: "PgRule028Message", Other: "执行计划存在笛卡尔积，建议检查表关联条件"}
)

// pg_031
var (
	PgRule031Desc       = &i18n.Message{ID: "PgRule031Desc", Other: "不建议创建冗余索引"}
	PgRule031Annotation = &i18n.Message{ID: "PgRule031Annotation", Other: "冗余索引会浪费存储空间，增加数据修改的开销，并且可能降低查询性能"}
	PgRule031Message    = &i18n.Message{ID: "PgRule031Message", Other: "不建议创建冗余索引"}
)

// pg_032
var (
	PgRule032Desc       = &i18n.Message{ID: "PgRule032Desc", Other: "禁止使用没有WHERE条件或者WHERE条件恒为TRUE的SQL"}
	PgRule032Annotation = &i18n.Message{ID: "PgRule032Annotation", Other: "SQL缺少WHERE条件在执行时会进行全表扫描产生额外开销，建议在大数据量高并发环境下开启，避免影响数据库查询性能"}
	PgRule032Message    = &i18n.Message{ID: "PgRule032Message", Other: "禁止使用没有WHERE条件或者WHERE条件恒为TRUE的SQL"}
)

// pg_033
var (
	PgRule033Desc       = &i18n.Message{ID: "PgRule033Desc", Other: "不建议使用标量子查询"}
	PgRule033Annotation = &i18n.Message{ID: "PgRule033Annotation", Other: "在大数据量时，标量子查询影响性能"}
	PgRule033Message    = &i18n.Message{ID: "PgRule033Message", Other: "不建议使用标量子查询"}
)

// pg_034
var (
	PgRule034Desc       = &i18n.Message{ID: "PgRule034Desc", Other: "不建议对多个字段使用OR条件查询"}
	PgRule034Annotation = &i18n.Message{ID: "PgRule034Annotation", Other: "对多个字段使用OR条件查询可能会导致SQL无法使用正确的索引"}
	PgRule034Message    = &i18n.Message{ID: "PgRule034Message", Other: "不建议对多个字段使用OR条件查询"}
)

// pg_035
var (
	PgRule035Desc       = &i18n.Message{ID: "PgRule035Desc", Other: "每个对象名称前面必须带有属主"}
	PgRule035Annotation = &i18n.Message{ID: "PgRule035Annotation", Other: "按照开发规范，每个对象名称前面必须带有属主。"}
	PgRule035Message    = &i18n.Message{ID: "PgRule035Message", Other: "每个对象名称前面必须带有属主"}
)

// pg_036
var (
	PgRule036Desc       = &i18n.Message{ID: "PgRule036Desc", Other: "不建议删除对象"}
	PgRule036Annotation = &i18n.Message{ID: "PgRule036Annotation", Other: "删除对象，存在风险"}
	PgRule036Message    = &i18n.Message{ID: "PgRule036Message", Other: "不建议删除对象"}
)

// pg_037
var (
	PgRule037Desc       = &i18n.Message{ID: "PgRule037Desc", Other: "不建议删除列"}
	PgRule037Annotation = &i18n.Message{ID: "PgRule037Annotation", Other: "语句中有删除列操作，存在一定风险"}
	PgRule037Message    = &i18n.Message{ID: "PgRule037Message", Other: "不建议删除列"}
)

// pg_038
var (
	PgRule038Desc       = &i18n.Message{ID: "PgRule038Desc", Other: "查询语句不建议使用UNION"}
	PgRule038Annotation = &i18n.Message{ID: "PgRule038Annotation", Other: "UNION需要对数据排序去重，可能会影响性能"}
	PgRule038Message    = &i18n.Message{ID: "PgRule038Message", Other: "查询语句不建议使用UNION"}
)

// pg_039
var (
	PgRule039Desc       = &i18n.Message{ID: "PgRule039Desc", Other: "同一张表的索引字段不建议超过阈值"}
	PgRule039Annotation = &i18n.Message{ID: "PgRule039Annotation", Other: "过多的索引会导致额外的存储开销、更新和删除操作的性能下降，增加数据库维护的成本。具体规则阈值可以根据业务需求调整。"}
	PgRule039Message    = &i18n.Message{ID: "PgRule039Message", Other: "同一张表的索引字段不建议超过阈值%v"}
	PgRule039Params1    = &i18n.Message{ID: "PgRule039Params1", Other: "最大索引字段数"}
)

// pg_040
var (
	PgRule040Desc       = &i18n.Message{ID: "PgRule040Desc", Other: "主键建议使用自增"}
	PgRule040Annotation = &i18n.Message{ID: "PgRule040Annotation", Other: "主键自增可以提高查询效率，减少存储空间占用"}
	PgRule040Message    = &i18n.Message{ID: "PgRule040Message", Other: "主键建议使用自增"}
)

// pg_041
var (
	PgRule041Desc       = &i18n.Message{ID: "PgRule041Desc", Other: "表连接数不建议超过阈值"}
	PgRule041Annotation = &i18n.Message{ID: "PgRule041Annotation", Other: "表关联越多，意味着各种驱动关系组合就越多，比较各种结果集的执行成本的代价也就越高，进而SQL查询性能会大幅度下降；具体规则阈值可以根据业务需求调整。"}
	PgRule041Message    = &i18n.Message{ID: "PgRule041Message", Other: "表连接数不建议超过阈值%d"}
	PgRule041Params1    = &i18n.Message{ID: "PgRule041Params1", Other: "连接表的最大个数可配置，默认：3"}
)

// pg_042
var (
	PgRule042Desc       = &i18n.Message{ID: "PgRule042Desc", Other: "在WHERE条件中禁止对索引列使用函数或表达式"}
	PgRule042Annotation = &i18n.Message{ID: "PgRule042Annotation", Other: "SQL语句在索引列上使用函数或表达式，会导致无法使用索引，不合理的消耗数据库资源，影响业务稳定运行。"}
	PgRule042Message    = &i18n.Message{ID: "PgRule042Message", Other: "在WHERE条件中禁止对索引列使用函数或表达式"}
)

// pg_043
var (
	PgRule043Desc       = &i18n.Message{ID: "PgRule043Desc", Other: "不建议在查询列上使用表达式"}
	PgRule043Annotation = &i18n.Message{ID: "PgRule043Annotation", Other: "SQL语句在查询条件列上使用表达式，可能导致无法使用索引，不合理的消耗数据库资源，影响业务稳定运行。"}
	PgRule043Message    = &i18n.Message{ID: "PgRule043Message", Other: "不建议在查询列上使用表达式"}
)

// pg_044
var (
	PgRule044Desc       = &i18n.Message{ID: "PgRule044Desc", Other: "多表关联时，不建议在WHERE条件中对不同表的字段使用OR条件"}
	PgRule044Annotation = &i18n.Message{ID: "PgRule044Annotation", Other: "多表关联时，在WHERE条件中对不同表的字段使用OR条件可能会导致SQL无法使用正确的索引"}
	PgRule044Message    = &i18n.Message{ID: "PgRule044Message", Other: "多表关联时，不建议在WHERE条件中对不同表的字段使用OR条件"}
)

// pg_045
var (
	PgRule045Desc       = &i18n.Message{ID: "PgRule045Desc", Other: "不建议使用全表扫描"}
	PgRule045Annotation = &i18n.Message{ID: "PgRule045Annotation", Other: "SQL语句执行计划中存在全表扫描，会导致不合理的消耗数据库资源，影响业务稳定运行。"}
	PgRule045Message    = &i18n.Message{ID: "PgRule045Message", Other: "不建议使用全表扫描"}
)

// pg_047
var (
	PgRule047Desc       = &i18n.Message{ID: "PgRule047Desc", Other: "不建议使用反向查询"}
	PgRule047Annotation = &i18n.Message{ID: "PgRule047Annotation", Other: "WHERE条件中使用!=、<>、NOT IN、NOT EXISTS、NOT LIKE以及NOT等，因为它们会使逻辑变得复杂，容易出错，且可能导致无法使用索引。"}
	PgRule047Message    = &i18n.Message{ID: "PgRule047Message", Other: "不建议使用反向查询"}
)

// pg_048
var (
	PgRule048Desc       = &i18n.Message{ID: "PgRule048Desc", Other: "建表DDL必须包含创建时间字段，类型为TIMESTAMP"}
	PgRule048Annotation = &i18n.Message{ID: "PgRule048Annotation", Other: "记录数据创建时间，有利于问题查找跟踪和检索数据，同时避免后期对数据生命周期管理不便"}
	PgRule048Message    = &i18n.Message{ID: "PgRule048Message", Other: "建表DDL必须包含创建时间字段，类型为TIMESTAMP"}
	PgRule048Params1    = &i18n.Message{ID: "PgRule048Params1", Other: "字段名称可配置，默认：created_time"}
)

// pg_049
var (
	PgRule049Desc       = &i18n.Message{ID: "PgRule049Desc", Other: "建表DDL必须包含更新时间字段，类型为TIMESTAMP"}
	PgRule049Annotation = &i18n.Message{ID: "PgRule049Annotation", Other: "记录数据创建时间，有利于问题查找跟踪和检索数据，同时避免后期对数据生命周期管理不便"}
	PgRule049Message    = &i18n.Message{ID: "PgRule049Message", Other: "建表DDL必须包含更新时间字段，类型为TIMESTAMP"}
	PgRule049Params1    = &i18n.Message{ID: "PgRule049Params1", Other: "字段名称可配置，默认：modified_time"}
)

// pg_050
var (
	PgRule050Desc       = &i18n.Message{ID: "PgRule050Desc", Other: "对象名称字符个数不建议超过阈值"}
	PgRule050Annotation = &i18n.Message{ID: "PgRule050Annotation", Other: "通过配置该规则可以规范指定业务的对象命名长度，具体长度可以自定义设置，默认最大长度：63个字符。"}
	PgRule050Message    = &i18n.Message{ID: "PgRule050Message", Other: "对象名称字符个数不建议超过阈值%v"}
	PgRule050Params1    = &i18n.Message{ID: "PgRule050Params1", Other: "最大字符个数可配置，默认63，检查以下对象：表、索引、函数、视图、序列、字段等"}
)

// pg_051
var (
	PgRule051Desc       = &i18n.Message{ID: "PgRule051Desc", Other: "外键字段上必须创建索引"}
	PgRule051Annotation = &i18n.Message{ID: "PgRule051Annotation", Other: "外键索引可以加速与外键相关的查询操作。"}
	PgRule051Message    = &i18n.Message{ID: "PgRule051Message", Other: "外键字段上必须创建索引"}
)

// pg_052
var (
	PgRule052Desc       = &i18n.Message{ID: "PgRule052Desc", Other: "建议索引字段的区分度大于阈值"}
	PgRule052Annotation = &i18n.Message{ID: "PgRule052Annotation", Other: "选择区分度高的字段作为索引，可快速定位数据；区分度太低，无法有效利用索引，甚至可能需要扫描大量数据页，拖慢SQL；具体规则阈值可以根据业务需求调整，默认值：70"}
	PgRule052Message    = &i18n.Message{ID: "PgRule052Message", Other: "建议索引字段的区分度大于阈值%v%%"}
	PgRule052Params1    = &i18n.Message{ID: "PgRule052Params1", Other: "最小区分度（百分比）"}
)

// pg_053
var (
	PgRule053Desc       = &i18n.Message{ID: "PgRule053Desc", Other: "索引最左侧的字段必须出现在查询条件内"}
	PgRule053Annotation = &i18n.Message{ID: "PgRule053Annotation", Other: "当查询条件包含索引的最左侧字段时，查询将最大程度地利用索引的有序性，提高查询性能。"}
	PgRule053Message    = &i18n.Message{ID: "PgRule053Message", Other: "索引最左侧的字段必须出现在查询条件内"}
)

// pg_054
var (
	PgRule054Desc       = &i18n.Message{ID: "PgRule054Desc", Other: "数据库对象名必须只包含小写字母，下划线，数字。且不能以数字开头"}
	PgRule054Annotation = &i18n.Message{ID: "PgRule054Annotation", Other: "通过配置该规则可以规范指定业务的数据对象命名规则"}
	PgRule054Message    = &i18n.Message{ID: "PgRule054Message", Other: "数据库对象名必须只包含小写字母，下划线，数字。且不能以数字开头"}
)

// pg_055
var (
	PgRule055Desc       = &i18n.Message{ID: "PgRule055Desc", Other: "绑定的变量个数不建议超过阈值"}
	PgRule055Annotation = &i18n.Message{ID: "PgRule055Annotation", Other: "过度使用绑定变量会增加查询的复杂度，从而降低查询性能。过度使用绑定变量还会增加维护成本。默认阈值:10000"}
	PgRule055Message    = &i18n.Message{ID: "PgRule055Message", Other: "绑定的变量个数不建议超过阈值%v"}
	PgRule055Params1    = &i18n.Message{ID: "PgRule055Params1", Other: "最大绑定变量个数可配置，默认值：100"}
)

// pg_056
var (
	PgRule056Desc       = &i18n.Message{ID: "PgRule056Desc", Other: "临时表必须使用固定前缀"}
	PgRule056Annotation = &i18n.Message{ID: "PgRule056Annotation", Other: "统一命名规范，有利于后期维护以及业务开发，检查创建临时表语句：CREATE TEMPORARY TABLE"}
	PgRule056Message    = &i18n.Message{ID: "PgRule056Message", Other: "临时表必须使用固定前缀%v"}
	PgRule056Params1    = &i18n.Message{ID: "PgRule056Params1", Other: "前缀可配置，默认：tmp_"}
)

// pg_057
var (
	PgRule057Desc       = &i18n.Message{ID: "PgRule057Desc", Other: "SQL语句中出现的关键字、保留字禁止使用大小写混合"}
	PgRule057Annotation = &i18n.Message{ID: "PgRule057Annotation", Other: "大小写混合的写法会导致代码的可读性降低和维护困难，从而造成误解或错误识别。"}
	PgRule057Message    = &i18n.Message{ID: "PgRule057Message", Other: "SQL语句中出现的关键字、保留字禁止使用大小写混合"}
)

// pg_059
var (
	PgRule059Desc       = &i18n.Message{ID: "PgRule059Desc", Other: "JOIN表时，关联字段的类型必须保持一致"}
	PgRule059Annotation = &i18n.Message{ID: "PgRule059Annotation", Other: "JOIN字段类型不一致会导致类型不匹配发生隐式转换，建议开启此规则，避免索引失效"}
	PgRule059Message    = &i18n.Message{ID: "PgRule059Message", Other: "JOIN表时，关联字段的类型必须保持一致"}
)

// pg_060
var (
	PgRule060Desc       = &i18n.Message{ID: "PgRule060Desc", Other: "禁止使用自定义函数"}
	PgRule060Annotation = &i18n.Message{ID: "PgRule060Annotation", Other: "自定义函数，维护较差，且依赖性高会导致SQL无法跨库使用"}
	PgRule060Message    = &i18n.Message{ID: "PgRule060Message", Other: "禁止使用自定义函数"}
)

// pg_061
var (
	PgRule061Desc       = &i18n.Message{ID: "PgRule061Desc", Other: "禁止使用存储过程"}
	PgRule061Annotation = &i18n.Message{ID: "PgRule061Annotation", Other: "存储过程在一定程度上会使程序难以调试和拓展，各种数据库的存储过程语法相差很大，给将来的数据库移植带来很大的困难，且会极大的增加出现BUG的概率"}
	PgRule061Message    = &i18n.Message{ID: "PgRule061Message", Other: "禁止使用存储过程"}
)

// pg_062
var (
	PgRule062Desc       = &i18n.Message{ID: "PgRule062Desc", Other: "同一张表索引个数建议不超过阈值"}
	PgRule062Annotation = &i18n.Message{ID: "PgRule062Annotation", Other: "在表上建立的每个索引都会增加存储开销，索引对于插入、删除、更新操作也会增加处理上的开销，太多与不充分、不正确的索引对性能都毫无益处；具体规则阈值可以根据业务需求调整，默认值：5"}
	PgRule062Message    = &i18n.Message{ID: "PgRule062Message", Other: "同一张表索引个数建议不超过阈值%v"}
	PgRule062Params1    = &i18n.Message{ID: "PgRule062Params1", Other: "最大索引个数，可配置，默认：5"}
)

// pg_063
var (
	PgRule063Desc       = &i18n.Message{ID: "PgRule063Desc", Other: "索引字段需要有非空约束"}
	PgRule063Annotation = &i18n.Message{ID: "PgRule063Annotation", Other: "索引字段上如果没有非空约束，则表记录与索引记录不会完全映射。"}
	PgRule063Message    = &i18n.Message{ID: "PgRule063Message", Other: "索引字段需要有非空约束"}
)

// pg_064
var (
	PgRule064Desc       = &i18n.Message{ID: "PgRule064Desc", Other: "单字段上的索引数量不建议超过阈值"}
	PgRule064Annotation = &i18n.Message{ID: "PgRule064Annotation", Other: "单字段上存在过多索引，一般情况下这些索引都是没有存在价值的；相反，还会降低数据增加删除时的性能，特别是对频繁更新的表来说，负面影响更大；具体规则阈值可以根据业务需求调整，默认值：2"}
	PgRule064Message    = &i18n.Message{ID: "PgRule064Message", Other: "单字段上的索引数量不建议超过阈值%v"}
	PgRule064Params1    = &i18n.Message{ID: "PgRule064Params1", Other: "单字段索引数量阈值，默认值：2"}
)

// pg_065
var (
	PgRule065Desc       = &i18n.Message{ID: "PgRule065Desc", Other: "使用分页查询时，避免使用偏移量"}
	PgRule065Annotation = &i18n.Message{ID: "PgRule065Annotation", Other: "当偏移量过大的时候，查询效率会很低，例如：LIMIT N OFFSET M或OFFSET M LIMIT N。因为需要执行大量的索引遍历、游标定位和分页计算操作。对于有大数据量的PostgreSQL表来说，使用OFFSET分页存在很严重的性能问题"}
	PgRule065Message    = &i18n.Message{ID: "PgRule065Message", Other: "使用分页查询时，避免使用偏移量"}
)

// pg_066
var (
	PgRule066Desc       = &i18n.Message{ID: "PgRule066Desc", Other: "数据库对象命名不建议大小写字母混合"}
	PgRule066Annotation = &i18n.Message{ID: "PgRule066Annotation", Other: "数据库对象命名规范，不推荐采用大小写混用的形式建议词语之间使用下划线连接，提高代码可读性"}
	PgRule066Message    = &i18n.Message{ID: "PgRule066Message", Other: "数据库对象命名不建议大小写字母混合"}
)

// pg_067
var (
	PgRule067Desc       = &i18n.Message{ID: "PgRule067Desc", Other: "UNIQUE索引必须使用固定前缀"}
	PgRule067Annotation = &i18n.Message{ID: "PgRule067Annotation", Other: "通过配置该规则可以规范指定业务的UNIQUE索引命名规则，具体命名规范可以自定义设置，默认提示值：uniq_"}
	PgRule067Message    = &i18n.Message{ID: "PgRule067Message", Other: "UNIQUE索引必须使用固定前缀%v"}
	PgRule067Params1    = &i18n.Message{ID: "PgRule067Params1", Other: "前缀可配置，默认：uniq_"}
)

// pg_068
var (
	PgRule068Desc       = &i18n.Message{ID: "PgRule068Desc", Other: "单条INSERT语句，建议批量插入不超过阈值"}
	PgRule068Annotation = &i18n.Message{ID: "PgRule068Annotation", Other: "避免大事务，以及降低发生回滚对业务的影响；具体规则阈值可以根据业务需求调整，默认值：100"}
	PgRule068Message    = &i18n.Message{ID: "PgRule068Message", Other: "单条INSERT语句，建议批量插入不超过阈值%v"}
	PgRule068Params1    = &i18n.Message{ID: "PgRule068Params1", Other: "单条INSERT最大插入行数，默认值：100"}
)

// pg_069
var (
	PgRule069Desc       = &i18n.Message{ID: "PgRule069Desc", Other: "不建议对表进行全索引扫描"}
	PgRule069Annotation = &i18n.Message{ID: "PgRule069Annotation", Other: "在数据量大的情况下索引全扫描严重影响SQL性能。"}
	PgRule069Message    = &i18n.Message{ID: "PgRule069Message", Other: "不建议对表进行全索引扫描"}
)

// pg_070
var (
	PgRule070Desc       = &i18n.Message{ID: "PgRule070Desc", Other: "WHERE条件内IN语句中的参数个数不能超过阈值"}
	PgRule070Annotation = &i18n.Message{ID: "PgRule070Annotation", Other: "当IN值过多时，有可能会导致查询进行全表扫描，使得PostgreSQL性能急剧下降；具体规则阈值可以根据业务需求调整，默认值：50"}
	PgRule070Message    = &i18n.Message{ID: "PgRule070Message", Other: "WHERE条件内IN语句中的参数个数不能超过阈值%v"}
	PgRule070Params1    = &i18n.Message{ID: "PgRule070Params1", Other: "IN语句中的参数个数阈值，默认值：50"}
)

// pg_071
var (
	PgRule071Desc       = &i18n.Message{ID: "PgRule071Desc", Other: "建议使用UNION ALL,替代UNION"}
	PgRule071Annotation = &i18n.Message{ID: "PgRule071Annotation", Other: "UNION会按照字段的顺序进行排序同时去重，UNION ALL只是简单的将两个结果合并后就返回，从效率上看，UNION ALL要比UNION快很多；如果合并的两个结果集中允许包含重复数据且不需要排序时的话，建议开启此规则，使用UNION ALL替代UNION"}
	PgRule071Message    = &i18n.Message{ID: "PgRule071Message", Other: "建议使用UNION ALL,替代UNION"}
)

// pg_072
var (
	PgRule072Desc       = &i18n.Message{ID: "PgRule072Desc", Other: "避免使用不必要的内置函数"}
	PgRule072Annotation = &i18n.Message{ID: "PgRule072Annotation", Other: "通过配置该规则可以指定业务中需要禁止使用的内置函数，使用内置函数可能会导致SQL无法走索引或者产生一些非预期的结果。实际需要禁用的函数可通过规则设置，默认函数集合为：sha1(),sha256(),sqrt(),md5()"}
	PgRule072Message    = &i18n.Message{ID: "PgRule072Message", Other: "避免使用不必要的内置函数%v"}
	PgRule072Params1    = &i18n.Message{ID: "PgRule072Params1", Other: "指定的函数集合（逗号分割）"}
)

// pg_073
var (
	PgRule073Desc       = &i18n.Message{ID: "PgRule073Desc", Other: "使用JOIN连接表查询建议不超过阈值"}
	PgRule073Annotation = &i18n.Message{ID: "PgRule073Annotation", Other: "表关联越多，意味着各种驱动关系组合就越多，比较各种结果集的执行成本的代价也就越高，进而SQL查询性能会大幅度下降；具体规则阈值可以根据业务需求调整，默认值：3"}
	PgRule073Message    = &i18n.Message{ID: "PgRule073Message", Other: "使用JOIN连接表查询建议不超过阈值%v"}
	PgRule073Params1    = &i18n.Message{ID: "PgRule073Params1", Other: "最大连接表个数"}
)

// pg_074
var (
	PgRule074Desc       = &i18n.Message{ID: "PgRule074Desc", Other: "禁止对长字段排序"}
	PgRule074Annotation = &i18n.Message{ID: "PgRule074Annotation", Other: "对例如VARCHAR(2000)这样的长字段进行ORDER BY、DISTINCT、GROUP BY、UNION之类的操作，会引发排序，有性能隐患，默认值：2000"}
	PgRule074Message    = &i18n.Message{ID: "PgRule074Message", Other: "禁止对长字段排序"}
	PgRule074Params1    = &i18n.Message{ID: "PgRule074Params1", Other: "长字段可配置，默认值：2000"}
)

// pg_075
var (
	PgRule075Desc       = &i18n.Message{ID: "PgRule075Desc", Other: "SELECT语句必须限制查询结果行数"}
	PgRule075Annotation = &i18n.Message{ID: "PgRule075Annotation", Other: "如果查询的扫描行数很大，可能会导致优化器选择错误的索引甚至不走索引；具体规则阈值可以根据业务需求调整，默认值：1000"}
	PgRule075Message    = &i18n.Message{ID: "PgRule075Message", Other: "SELECT语句必须限制查询结果行数，默认值：%v"}
	PgRule075Params1    = &i18n.Message{ID: "PgRule075Params1", Other: "查询结果最大行数"}
)

// pg_076
var (
	PgRule076Desc       = &i18n.Message{ID: "PgRule076Desc", Other: "不建议在ORDER BY语句中对多个不同条件使用不同方向的排序"}
	PgRule076Annotation = &i18n.Message{ID: "PgRule076Annotation", Other: "当ORDER BY子句中多个列的排序方向不一致时，通常会导致PostgreSQL无法利用单一索引来满足所有排序需求，可能需要进行排序操作或进行多个索引的合并操作，这会影响查询的性能。"}
	PgRule076Message    = &i18n.Message{ID: "PgRule076Message", Other: "不建议在ORDER BY语句中对多个不同条件使用不同方向的排序"}
)

// pg_077
var (
	PgRule077Desc       = &i18n.Message{ID: "PgRule077Desc", Other: "子查询嵌套层数不建议超过阈值"}
	PgRule077Annotation = &i18n.Message{ID: "PgRule077Annotation", Other: "子查询嵌套层数超过阈值，有些情况下，子查询并不能使用到索引。同时对于返回结果集比较大的子查询，会产生大量的临时表，消耗过多的CPU和IO资源，产生大量的慢查询，默认值: 3"}
	PgRule077Message    = &i18n.Message{ID: "PgRule077Message", Other: "子查询嵌套层数不建议超过阈值%v"}
	PgRule077Params1    = &i18n.Message{ID: "PgRule077Params1", Other: "子查询嵌套层次阈值，默认值：3"}
)

// pg_078
var (
	PgRule078Desc       = &i18n.Message{ID: "PgRule078Desc", Other: "禁止WHERE条件中使用与过滤字段不一致的数据类型"}
	PgRule078Annotation = &i18n.Message{ID: "PgRule078Annotation", Other: "WHERE条件中使用与过滤字段不一致的数据类型会引发隐式数据类型转换，导致查询有无法命中索引的风险，在高并发、大数据量的情况下，不走索引会使得数据库的查询性能严重下降"}
	PgRule078Message    = &i18n.Message{ID: "PgRule078Message", Other: "禁止WHERE条件中使用与过滤字段不一致的数据类型"}
)

// pg_079
var (
	PgRule079Desc       = &i18n.Message{ID: "PgRule079Desc", Other: "LIMIT查询建议使用ORDER BY"}
	PgRule079Annotation = &i18n.Message{ID: "PgRule079Annotation", Other: "没有ORDER BY的LIMIT会导致非确定性的结果可能与业务需求不符，这取决于执行计划"}
	PgRule079Message    = &i18n.Message{ID: "PgRule079Message", Other: "LIMIT查询建议使用ORDER BY"}
)

// pg_080
var (
	PgRule080Desc              = &i18n.Message{ID: "PgRule080Desc", Other: "DDL执行前存在事务未提交"}
	PgRule080Annotation        = &i18n.Message{ID: "PgRule080Annotation", Other: "检查DDL操作执行前是否存在对相关资源加锁的事务，避免DDL操作产生锁冲突导致锁等待或死锁问题"}
	PgRule080Message           = &i18n.Message{ID: "PgRule080Message", Other: "DDL执行前存在事务未提交"}
	PgRule080MessageLock       = &i18n.Message{ID: "PgRule080MessageLock", Other: "DDL执行前存在事务未提交, pg_locks中存在%d个事务加锁, 请人工确认"}
	PgRule080MessageSchemaLock = &i18n.Message{ID: "PgRule080MessageSchemaLock", Other: "DDL执行前存在事务未提交, pg_locks中存在%d个事务对schema[%s]资源加锁, 请人工确认"}
	PgRule080MessageTableLock  = &i18n.Message{ID: "PgRule080MessageTableLock", Other: "DDL执行前存在事务未提交, pg_locks中存在%d个事务对表[%s.%s]加锁"}
)
