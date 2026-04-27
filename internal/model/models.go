package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User stores admin account data.
type User struct {
	ID           string     `gorm:"type:text;primaryKey"`
	Username     string     `gorm:"type:text;uniqueIndex;not null"`
	PasswordHash string     `gorm:"type:text;not null"`
	Role         string     `gorm:"type:text;not null;default:'admin'"`
	LastLoginAt  *time.Time `gorm:"type:datetime"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate assigns UUID values.
func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}

// NavGroup stores a link group.
type NavGroup struct {
	ID        string    `gorm:"type:text;primaryKey"`
	Name      string    `gorm:"type:text;not null"`
	SortOrder int       `gorm:"not null;default:0"`
	GridCols  int       `gorm:"not null;default:0"`
	GridRows  int       `gorm:"not null;default:0"`
	Lang      string    `gorm:"-"`
	CSRFToken string    `gorm:"-"`
	NavLinks  []NavLink `gorm:"foreignKey:GroupID;references:ID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate assigns UUID values.
func (g *NavGroup) BeforeCreate(_ *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	return nil
}

// NavLink stores a link item.
type NavLink struct {
	ID                     string     `gorm:"type:text;primaryKey"`
	GroupID                string     `gorm:"type:text;index;not null"`
	Title                  string     `gorm:"type:text;not null"`
	URL                    string     `gorm:"type:text;not null"`
	Description            string     `gorm:"type:text"`
	Icon                   string     `gorm:"type:text"`
	IconCachedPath         string     `gorm:"type:text"`
	ThemeAccentColor       string     `gorm:"type:text"`
	ThemeBgStartColor      string     `gorm:"type:text"`
	ThemeBgEndColor        string     `gorm:"type:text"`
	ThemeBorderColor       string     `gorm:"type:text"`
	ThemeTextColor         string     `gorm:"type:text"`
	IconStatus             string     `gorm:"type:text;not null;default:'pending';index"`
	IconLastCheckedAt      *time.Time `gorm:"type:datetime"`
	IconNextCheckAt        *time.Time `gorm:"type:datetime;index"`
	IconFailCount          int        `gorm:"not null;default:0"`
	ThumbnailCachedPath    string     `gorm:"type:text"`
	ThumbnailStatus        string     `gorm:"type:text;not null;default:'pending';index"`
	ThumbnailLastCheckedAt *time.Time `gorm:"type:datetime"`
	ThumbnailNextCheckAt   *time.Time `gorm:"type:datetime;index"`
	ThumbnailFailCount     int        `gorm:"not null;default:0"`
	OpenInNew              bool       `gorm:"not null;default:true"`
	SortOrder              int        `gorm:"not null;default:0"`
	Lang                   string     `gorm:"-"`
	CSRFToken              string     `gorm:"-"`
	UseThumbnailBackground bool       `gorm:"-"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate assigns UUID values.
func (l *NavLink) BeforeCreate(_ *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	if l.IconStatus == "" {
		l.IconStatus = "pending"
	}
	if l.ThumbnailStatus == "" {
		l.ThumbnailStatus = "pending"
	}
	return nil
}

// FaviconCache stores one cached favicon per normalized domain.
type FaviconCache struct {
	Domain            string     `gorm:"type:text;primaryKey"`
	IconCachedPath    string     `gorm:"type:text"`
	ThemeAccentColor  string     `gorm:"type:text"`
	ThemeBgStartColor string     `gorm:"type:text"`
	ThemeBgEndColor   string     `gorm:"type:text"`
	ThemeBorderColor  string     `gorm:"type:text"`
	ThemeTextColor    string     `gorm:"type:text"`
	IconStatus        string     `gorm:"type:text;not null;default:'pending';index"`
	IconLastCheckedAt *time.Time `gorm:"type:datetime"`
	IconNextCheckAt   *time.Time `gorm:"type:datetime;index"`
	IconFailCount     int        `gorm:"not null;default:0"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ThumbnailCache stores one cached website thumbnail per normalized domain.
type ThumbnailCache struct {
	Domain                 string     `gorm:"type:text;primaryKey"`
	ThumbnailCachedPath    string     `gorm:"type:text"`
	ThumbnailStatus        string     `gorm:"type:text;not null;default:'pending';index"`
	ThumbnailLastCheckedAt *time.Time `gorm:"type:datetime"`
	ThumbnailNextCheckAt   *time.Time `gorm:"type:datetime;index"`
	ThumbnailFailCount     int        `gorm:"not null;default:0"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// Setting stores key-value settings.
type Setting struct {
	ID        string `gorm:"type:text;primaryKey"`
	Key       string `gorm:"type:text;uniqueIndex;not null"`
	Value     string `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BeforeCreate assigns UUID values.
func (s *Setting) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
