package database

const (
	ActionInsert     = "insert"
	ActionUpdate     = "update"
	ActionUpsert     = "upsert"
	ActionDelete     = "delete"
	ActionBulkInsert = "bulk_insert"
	ActionBulkUpdate = "bulk_update"
	ActionUpdateById = "update_by_id"
	ActionDeleteById = "delete_by_id"

	MySQLEngine      = "mysql"
	NRMySQLEngine    = "nrmysql"
	PostgresEngine   = "postgres"
	NRPostgresEngine = "nrpostgres"

	LockShare  = "share"
	LockUpdate = "update"

	NewRelicAttributeQuery  = "query"
	NewRelicAttributeParams = "params"
)

var (
	postgresEngineGroup = map[string]bool{
		PostgresEngine:   true,
		NRPostgresEngine: true,
	}
	mySQLEngineGroup = map[string]bool{
		MySQLEngine:   true,
		NRMySQLEngine: true,
	}
)
