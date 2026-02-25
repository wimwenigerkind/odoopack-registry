package models

import "gorm.io/gorm"

type Addon struct {
	gorm.Model
	Name     string         `gorm:"uniqueIndex" json:"name"`
	Versions []AddonVersion `json:"versions"`
}

type AddonVersion struct {
	Version    string `json:"version"`
	Type       string `json:"type"`
	Repository string `json:"repository"`

	AddonID uint `json:"-"`
}
