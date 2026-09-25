package model

import "time"

type User struct {
	ID              int       `gorm:"primaryKey" json:"id"`
	IP              string    `gorm:"type:varchar(233);default:''" json:"-"`
	Moemoepoint     int       `gorm:"default:0" json:"moemoepoint"`
	DailyImageCount int       `gorm:"default:0" json:"-"`
	DailyCheckIn    int       `gorm:"default:0" json:"-"`
	DailyUploadSize int64     `gorm:"default:0" json:"-"`
	LastLoginTime   string    `gorm:"default:''" json:"-"`
	Created         time.Time `gorm:"autoCreateTime" json:"created"`
	Updated         time.Time `gorm:"autoUpdateTime" json:"updated"`
}

func (User) TableName() string { return "user" }
