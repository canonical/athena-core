package db

import (
	"github.com/go-orm/gorm"
	"time"
)

type File struct {
	gorm.Model

	Created      time.Time `gorm:"autoCreateTime"` // Use unix seconds as creating time
	DispatchedAt time.Time
	Dispatched   bool   `gorm:"default:false"`
	Path         string `gorm:"primary_key"`
	Reports      []Report
}

type Report struct {
	gorm.Model

	Created        time.Time `gorm:"autoCreateTime"`
	Commented      bool      `gorm:"default:false"`
	UploadLocation string
	Name           string
	FileID         uint
	CaseID         string
}
