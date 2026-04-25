package storage

import (
	"strings"
	"time"

	"github.com/fisk086/sya/internal/model"
	"gorm.io/gorm"
)

// 使用 Find+Limit 而非 First，避免「用户名/邮箱可用」时的预期空结果触发 GORM 默认的 record not found 日志。
func firstUserByWhere (db *gorm.DB, cond string, arg any) (*model.User, error) {
	var users []model.User
	err := db.Where(cond, arg).Limit(1).Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &users[0], nil
}

type UserStore interface {
	GetUserByID(id int64) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	GetUserByEmail(email string) (*model.User, error)
	GetUserByLarkOpenID(openID string) (*model.User, error)
	CreateUser(user *model.User) error
	UpdateUser(user *model.User) error
	UpdateLastLogin(id int64, lastLogin time.Time) error
	ListUsers(page, pageSize int) ([]model.User, int64, error)
	DeleteUser(id int64) error
}

type GORMUserStore struct {
	db *gorm.DB
}

func NewGORMUserStore(db *gorm.DB) *GORMUserStore {
	return &GORMUserStore{db: db}
}

func (s *GORMUserStore) GetUserByID(id int64) (*model.User, error) {
	return firstUserByWhere(s.db, "id = ?", id)
}

func (s *GORMUserStore) GetUserByUsername(username string) (*model.User, error) {
	return firstUserByWhere(s.db, "username = ?", username)
}

func (s *GORMUserStore) GetUserByEmail(email string) (*model.User, error) {
	return firstUserByWhere(s.db, "email = ?", email)
}

func (s *GORMUserStore) GetUserByLarkOpenID(openID string) (*model.User, error) {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return firstUserByWhere(s.db, "lark_open_id = ?", openID)
}

func (s *GORMUserStore) CreateUser(user *model.User) error {
	return s.db.Create(user).Error
}

func (s *GORMUserStore) UpdateUser(user *model.User) error {
	return s.db.Save(user).Error
}

func (s *GORMUserStore) UpdateLastLogin(id int64, lastLogin time.Time) error {
	return s.db.Model(&model.User{}).Where("id = ?", id).Update("last_login", lastLogin).Error
}

func (s *GORMUserStore) ListUsers(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	s.db.Model(&model.User{}).Count(&total)

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	err := s.db.Order("id desc").Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

func (s *GORMUserStore) DeleteUser(id int64) error {
	return s.db.Where("id = ?", id).Delete(&model.User{}).Error
}
