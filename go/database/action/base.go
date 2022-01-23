package action

import (
	"github.com/kodekoding/phastos/go/common"
	"github.com/kodekoding/phastos/go/database"
)

type Base struct {
	BaseWrites
	common.ReadRepo
}

type baseAction struct {
	db           *database.SQL
	tableName    string
	isSoftDelete bool
}

func NewBase(db *database.SQL, tableName string, isSoftDelete ...bool) *Base {
	sofDelete := true
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &Base{
		BaseWrites: NewBaseWrite(db, tableName, sofDelete),
		ReadRepo:   NewBaseRead(db, tableName, sofDelete),
	}
}
