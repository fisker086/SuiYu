package storage

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type GORMDB struct {
	DB        *gorm.DB
	UserStore *GORMUserStore
}

func NewGORMDB(dsn string) (*GORMDB, error) {
	logLevel := gormlogger.Warn
	if os.Getenv("GIN_MODE") == "debug" || os.Getenv("DEBUG") == "1" {
		logLevel = gormlogger.Info
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return &GORMDB{
		DB:        db,
		UserStore: NewGORMUserStore(db),
	}, nil
}

func (g *GORMDB) AutoMigrate() error {
	return g.DB.AutoMigrate(
		&model.User{},
		&model.Schedule{},
		&model.ScheduleExecution{},
		&model.TokenUsage{},
	)
}

func (g *GORMDB) SeedDefaultAdmin() error {
	var count int64
	g.DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		return nil
	}

	adminPassword := os.Getenv("ADMIN_DEFAULT_PASSWORD")
	if adminPassword == "" {
		return fmt.Errorf("ADMIN_DEFAULT_PASSWORD environment variable is required for seeding default admin")
	}

	adminWhitelist := os.Getenv("ADMIN_WHITELIST")
	var adminUsernames []string
	if adminWhitelist != "" {
		for _, u := range strings.Split(adminWhitelist, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				adminUsernames = append(adminUsernames, u)
			}
		}
	}

	if len(adminUsernames) == 0 {
		adminUsernames = []string{"admin"}
	}

	for _, username := range adminUsernames {
		admin := &model.User{
			Username:    username,
			Email:       username + "@aiops.local",
			FullName:    ptrStr("Administrator"),
			Status:      model.UserStatusActive,
			IsSuperuser: true,
			IsAdmin:     true,
		}
		if err := admin.SetPassword(adminPassword); err != nil {
			logger.Error("failed to set password for admin", "username", username, "err", err)
			continue
		}

		if err := g.UserStore.CreateUser(admin); err != nil {
			logger.Error("failed to seed admin user", "username", username, "err", err)
			continue
		}
		logger.Info("seeded admin user", "username", username, "is_admin", true)
	}
	return nil
}

// ApplyAdminWhitelist 在启动时把 ADMIN_WHITELIST 里列出的「用户名」在数据库中设为管理员。
// 用于已注册用户：仅空库时的 SeedDefaultAdmin 不会更新他们，需靠本函数同步 .env。
// 只提升、不降级；名单外用户保持原 is_admin。
func (g *GORMDB) ApplyAdminWhitelist(whitelist string) error {
	whitelist = strings.TrimSpace(whitelist)
	if whitelist == "" {
		return nil
	}
	for _, u := range strings.Split(whitelist, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		res := g.DB.Model(&model.User{}).Where("username = ?", u).Updates(map[string]any{
			"is_admin":     true,
			"is_superuser": true,
		})
		if res.Error != nil {
			return fmt.Errorf("apply admin whitelist for %q: %w", u, res.Error)
		}
		if res.RowsAffected > 0 {
			logger.Info("applied ADMIN_WHITELIST: user is now admin", "username", u)
		} else {
			logger.Warn("ADMIN_WHITELIST username not found in DB (seed only creates empty DB; register this username first)", "username", u)
		}
	}
	return nil
}

func ptrStr(s string) *string {
	return &s
}
