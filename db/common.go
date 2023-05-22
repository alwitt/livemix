package db

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

/*
GetSqliteDialector define Sqlite GORM dialector

	@param dbFile string - Sqlite DB file
	@return GORM sqlite dialector
*/
func GetSqliteDialector(dbFile string) gorm.Dialector {
	return sqlite.Open(fmt.Sprintf("%s?_foreign_keys=on", dbFile))
}

/*
GetInMemSqliteDialector define a in-memory Sqlite GORM dialector

	@param dbName string - in-memory Sqlite DB name
	@return GORM sqlite dialector
*/
func GetInMemSqliteDialector(dbName string) gorm.Dialector {
	return sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared&_foreign_keys=on", dbName))
}
