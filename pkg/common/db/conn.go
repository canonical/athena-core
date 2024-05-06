package db

import (
	"fmt"

	"github.com/canonical/athena-core/pkg/config"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func GetDBConn(cfg *config.Config) (*gorm.DB, error) {
	var dBDriver gorm.Dialector = nil
	switch cfg.Db.Dialect {
	case "sqlite":
		dBDriver = sqlite.Open(cfg.Db.DSN)
	case "mysql":
		dBDriver = mysql.Open(cfg.Db.DSN)
	default:
		return nil, fmt.Errorf("unknown database dialect %s", cfg.Db.Dialect)
	}
	dbInstance, err := gorm.Open(dBDriver)
	if err != nil {
		return nil, err
	}

	switch cfg.Db.Dialect {
	case "sqlite":
		log.Debugln("Will not change collation")
		dbInstance.AutoMigrate(File{}, Report{}, Script{})
	case "mysql":
		var lockName = "migrate_lock"
		var timeout = 10 // seconds
		var lock int
		dbInstance.Raw("SELECT GET_LOCK(?, ?)", lockName, timeout).Scan(&lock)
		if lock == 1 {
			if !dbInstance.Migrator().HasColumn(&File{}, "Path") {
				log.Debugln("Changing collation to UTF-8")
				dbInstance.AutoMigrate(File{}, Report{}, Script{})
				err = dbInstance.Exec("ALTER TABLE files MODIFY Path VARCHAR(10240) CHARACTER SET utf8 COLLATE utf8_general_ci").Error
				if err != nil {
					log.Errorln("Could not change collation of files table")
				}
			}
			dbInstance.Exec("DO RELEASE_LOCK(?)", lockName)
		} else {
			log.Errorln("Could not get lock on database")
		}
	}
	return dbInstance, nil
}
