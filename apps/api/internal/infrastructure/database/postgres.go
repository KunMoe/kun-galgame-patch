package database

import (
	"log"
	"log/slog"
	"os"
	"time"

	"kun-galgame-patch-api/pkg/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewPostgres(cfg config.DatabaseConfig, mode string) *gorm.DB {
	logLevel := gormlogger.Info
	if mode == "prod" {
		logLevel = gormlogger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger: gormlogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gormlogger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logLevel,
			// First() on a missing row is how every lookup here says "absent".
			// Logged, that was 35,749 error blocks in 12 hours of prod, nearly
			// all for catalog works that have no local page.
			IgnoreRecordNotFoundError: true,
			Colorful:                  mode != "prod",
		}),
	})
	if err != nil {
		panic("failed to connect to database: " + err.Error())
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get underlying sql.DB: " + err.Error())
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)

	slog.Info("PostgreSQL connected")
	return db
}
