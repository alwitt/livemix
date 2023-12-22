package db

import (
	"fmt"
	"strings"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/*
GetSqliteDialector define Sqlite GORM dialector

	@param dbFile string - Sqlite DB file
	@return GORM sqlite dialector
*/
func GetSqliteDialector(dbFile string) gorm.Dialector {
	return sqlite.Open(fmt.Sprintf("%s?cache=shared&_foreign_keys=on", dbFile))
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

// ConnectionManager manages connections and transactions with a DB
type ConnectionManager interface {
	/*
		NewTransaction start and get handle to a new transaction

			@returns new transaction
	*/
	NewTransaction() *gorm.DB

	/*
		Commit Commit all changes within a transaction

			@param session *gorm.DB - the transaction session
	*/
	Commit(session *gorm.DB)

	/*
		Rollback revert all changes within a transaction

			@param session *gorm.DB - the transaction session
	*/
	Rollback(session *gorm.DB)

	/*
		NewPersistanceManager define a new DB access manager

			@returns new manager
	*/
	NewPersistanceManager() PersistenceManager
}

type connectionManagerImpl struct {
	goutils.Component
	db *gorm.DB
}

/*
NewSQLConnection define a new DB connection and transactions manager

	@param dbDialector gorm.Dialector - GORM SQL dialector
	@param logLevel logger.LogLevel - SQL log level
	@returns new manager
*/
func NewSQLConnection(
	dbDialector gorm.Dialector, logLevel logger.LogLevel,
) (ConnectionManager, error) {
	db, err := gorm.Open(dbDialector, &gorm.Config{
		Logger:                 logger.Default.LogMode(logLevel),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// Prepare the databases
	if err := db.AutoMigrate(&videoSource{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&liveStreamVideoSegment{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&segmentToRecordingAssociation{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&recordingSession{}, &recordingVideoSegment{}); err != nil {
		return nil, err
	}

	logTags := log.Fields{"module": "db", "component": "connection", "instance": dbDialector.Name()}
	return &connectionManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		}, db: db,
	}, nil
}

func (c *connectionManagerImpl) NewTransaction() *gorm.DB {
	return c.db.Begin()
}

func (c *connectionManagerImpl) Commit(session *gorm.DB) {
	session.Commit()
}

func (c *connectionManagerImpl) Rollback(session *gorm.DB) {
	session.Rollback()
}

func (c *connectionManagerImpl) NewPersistanceManager() PersistenceManager {
	return newManager(c)
}
