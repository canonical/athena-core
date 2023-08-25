package db

import (
    "github.com/canonical/athena-core/pkg/config"
    "github.com/go-orm/gorm"
    _ "github.com/go-sql-driver/mysql"
)

func GetDBConn(cfg *config.Config) (*gorm.DB, error) {
    dbInstance, err := gorm.Open(cfg.Db.Dialect, cfg.Db.DSN)
    if err != nil {
        return nil, err
    }

    dbInstance.AutoMigrate(File{}, Report{}, Script{})
    return dbInstance, nil
}
