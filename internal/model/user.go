package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username       string     `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email          string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	HashedPassword string     `gorm:"size:255;not null;column:hashed_password" json:"-"`
	FullName       *string    `gorm:"size:100" json:"full_name,omitempty"`
	AvatarURL      *string    `gorm:"size:512" json:"avatar_url,omitempty"`
	Status         UserStatus `gorm:"size:20;default:active" json:"status"`
	IsSuperuser    bool       `gorm:"default:false" json:"is_superuser"`
	IsAdmin        bool       `gorm:"default:false" json:"is_admin"`
	LarkOpenID     *string    `gorm:"size:64;index" json:"lark_open_id,omitempty"`
	LarkUnionID    *string    `gorm:"size:64;index" json:"lark_union_id,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.HashedPassword = string(hash)
	return nil
}

func (u *User) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(password))
	return err == nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Status == "" {
		u.Status = UserStatusActive
	}
	return nil
}
