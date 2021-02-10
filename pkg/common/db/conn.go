package db

import (
	"github.com/go-orm/gorm"
	"github.com/niedbalski/go-athena/pkg/config"
)

func GetDBConn(cfg *config.Config) (*gorm.DB, error) {
	dbInstance, err := gorm.Open(cfg.Db.Dialect, cfg.Db.DSN)
	if err != nil {
		return nil, err
	}

	dbInstance.AutoMigrate(File{}, Report{})
	return dbInstance, nil
}
