package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) Create(group *models.Group) error {
	return r.db.Create(group).Error
}

func (r *GroupRepository) GetByID(id uuid.UUID) (*models.Group, error) {
	var g models.Group
	err := r.db.Where("id = ?", id).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GroupRepository) GetByName(name string) (*models.Group, error) {
	var g models.Group
	err := r.db.Where("name = ?", name).First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GroupRepository) List() ([]models.Group, error) {
	var groups []models.Group
	err := r.db.Order("name").Find(&groups).Error
	return groups, err
}

func (r *GroupRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&models.GroupMembership{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&models.GroupAddonAccess{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Group{}, "id = ?", id).Error
	})
}

func (r *GroupRepository) AddMember(groupID, userID uuid.UUID) error {
	return r.db.Create(&models.GroupMembership{GroupID: groupID, UserID: userID}).Error
}

func (r *GroupRepository) EnsureMember(groupID, userID uuid.UUID) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.GroupMembership{GroupID: groupID, UserID: userID}).Error
}

func (r *GroupRepository) RemoveMember(groupID, userID uuid.UUID) error {
	return r.db.Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&models.GroupMembership{}).Error
}

func (r *GroupRepository) ListMembers(groupID uuid.UUID) ([]models.GroupMembership, error) {
	var members []models.GroupMembership
	err := r.db.Where("group_id = ?", groupID).Find(&members).Error
	return members, err
}

func (r *GroupRepository) GrantAddonAccess(groupID, addonID uuid.UUID) error {
	access := models.GroupAddonAccess{GroupID: groupID, AddonID: addonID}
	return r.db.FirstOrCreate(&access, "group_id = ? AND addon_id = ?", groupID, addonID).Error
}

func (r *GroupRepository) RevokeAddonAccess(groupID, addonID uuid.UUID) error {
	return r.db.Where("group_id = ? AND addon_id = ?", groupID, addonID).
		Delete(&models.GroupAddonAccess{}).Error
}

func (r *GroupRepository) ListAddonAccess(groupID uuid.UUID) ([]models.GroupAddonAccess, error) {
	var access []models.GroupAddonAccess
	err := r.db.Where("group_id = ?", groupID).Find(&access).Error
	return access, err
}

func (r *GroupRepository) UserCanReadAddon(userID, addonID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.GroupAddonAccess{}).
		Joins("JOIN group_memberships gm ON gm.group_id = group_addon_accesses.group_id").
		Where("gm.user_id = ? AND group_addon_accesses.addon_id = ?", userID, addonID).
		Count(&count).Error
	return count > 0, err
}
