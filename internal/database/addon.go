package database

import "gorm.io/gorm"

// indexAddonModuleNames indexes the module name of every addon, the
// expression repository.AddonRepository matches depends against. An
// expression index needs no backfill and cannot drift from the name on a
// rename, which a stored column could. It runs after AutoMigrate, which
// creates the table but knows no expression indexes.
func indexAddonModuleNames(db *gorm.DB) error {
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_addons_module_name ON addons (translate(name, '/-', '__'))`).Error
}
