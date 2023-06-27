package db

import (
	"fmt"
	"strings"

	"github.com/alwitt/livemix/common"
	"gorm.io/driver/postgres"
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

/*
GetPostgresDialector define Postgres driver dialector

	@param config common.PostgresConfig - connection config
	@param password string - user password
	@returns GORM Postgres dialector
*/
func GetPostgresDialector(config common.PostgresConfig, password string) (gorm.Dialector, error) {
	/*
		Configuration has be affected by

		* Whether to use password
		* Whether to use SSL
	*/
	configParams := []string{
		fmt.Sprintf("host=%s", config.Host),
		fmt.Sprintf("port=%d", config.Port),
		fmt.Sprintf("dbname=%s", config.Database),
		fmt.Sprintf("user=%s", config.User),
	}
	// When password is specified
	if password != "" {
		configParams = append(configParams, fmt.Sprintf("password=%s", password))
	}
	// When using SSL
	if config.SSL.Enabled {
		if config.SSL.CAFile == nil {
			return nil, fmt.Errorf("can't connect to Postgres with SSL without specific CA cert")
		}
		configParams = append(configParams, []string{
			"sslmode=verify-full",
			fmt.Sprintf("sslrootcert=%s", *config.SSL.CAFile),
		}...)
	}
	// Build the complete dialectic string
	return postgres.Open(strings.Join(configParams, " ")), nil
}
