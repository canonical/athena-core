package db

import (
	"fmt"

	"github.com/canonical/athena-core/pkg/config"
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

	dbInstance.AutoMigrate(File{}, Report{}, Script{})
	return dbInstance, nil
}
