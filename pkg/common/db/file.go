package db

import (
    "github.com/go-orm/gorm"
    "time"
)

type File struct {
    gorm.Model

    Created    time.Time `gorm:"autoCreateTime"` // Use unix seconds as creating time
    Dispatched bool      `gorm:"default:false"`
    Path       string    `gorm:"primary_key"`
    Reports    []Report
}

type Report struct {
    gorm.Model

    Created    time.Time `gorm:"<-:create"`
    Commented  bool      `gorm:"default:false"`
    Subscriber string
    Name       string
    FileID     uint
    FilePath   string
    CaseID     string
    Scripts    []Script
}

type Script struct {
    gorm.Model

    Output         string `gorm:"type:text"`
    Name           string
    UploadLocation string
    ReportID       uint
}
