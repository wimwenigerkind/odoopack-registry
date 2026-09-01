package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RepoRepository struct {
	db *gorm.DB
}

func NewRepoRepository(db *gorm.DB) *RepoRepository {
	return &RepoRepository{db: db}
}

func (r *RepoRepository) Create(repo *models.Repo) error {
	return r.db.Create(repo).Error
}

func (r *RepoRepository) FindOrCreate(repo *models.Repo) (bool, error) {
	res := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(repo)
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 1 {
		return true, nil
	}
	var existing models.Repo
	if err := r.db.Where("git_url = ?", repo.GitURL).First(&existing).Error; err != nil {
		return false, err
	}
	*repo = existing
	return false, nil
}

func (r *RepoRepository) GetByID(id uuid.UUID) (*models.Repo, error) {
	var repo models.Repo
	err := r.db.Preload("Owner").Preload("Integration").Preload("Addons").Where("id = ?", id).First(&repo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func (r *RepoRepository) GetByGitURL(gitURL string) (*models.Repo, error) {
	var repo models.Repo
	err := r.db.Where("git_url = ?", gitURL).First(&repo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func (r *RepoRepository) ListByOwner(ownerID uuid.UUID) ([]models.Repo, error) {
	var repos []models.Repo
	err := r.db.
		Preload("Addons").
		Preload("Integration").
		Where("owner_id = ?", ownerID).
		Order("created_at DESC").
		Find(&repos).Error
	return repos, err
}

func (r *RepoRepository) Update(repo *models.Repo) error {
	return r.db.Model(repo).Select("git_url", "default_branch", "integration_id").Updates(map[string]any{
		"git_url":        repo.GitURL,
		"default_branch": repo.DefaultBranch,
		"integration_id": repo.IntegrationID,
	}).Error
}

func (r *RepoRepository) Delete(id, ownerID uuid.UUID) error {
	res := r.db.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Repo{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RepoRepository) CountAddons(repoID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Addon{}).Where("repo_id = ?", repoID).Count(&count).Error
	return count, err
}
