package models

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	Base
	Name string `gorm:"uniqueIndex;not null" json:"name"`
}

type GroupMembership struct {
	GroupID   uuid.UUID `gorm:"primaryKey;type:uuid" json:"group_id"`
	UserID    uuid.UUID `gorm:"primaryKey;type:uuid" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type GroupAddonAccess struct {
	GroupID   uuid.UUID `gorm:"primaryKey;type:uuid" json:"group_id"`
	AddonID   uuid.UUID `gorm:"primaryKey;type:uuid" json:"addon_id"`
	CreatedAt time.Time `json:"created_at"`
}
