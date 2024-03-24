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

	MySQLEngine    = "mysql"
	PostgresEngine = "postgres"

	LockShare  = "share"
	LockUpdate = "update"
)
