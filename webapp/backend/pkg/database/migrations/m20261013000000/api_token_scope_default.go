package m20261013000000

import (
	"fmt"

	"gorm.io/gorm"
)

// ApiToken is api_tokens as m20260225000000 created it, minus the default on scope. Only the
// SQLite table rebuild reads it, to know the new definition of scope.
type ApiToken struct {
	Scope string
}

func (ApiToken) TableName() string {
	return "api_tokens"
}

// Migrate drops the 'full' default from api_tokens.scope. 'full' is not a valid scope, so the API
// rejects any token holding it; a token must now be created with an explicit scope. Rows that
// already hold 'full' are left as they are: they cannot authenticate today, and rewriting them to
// a valid scope would grant access nobody approved. An administrator can delete them.
//
// SQLite cannot change a column default in place, so AlterColumn rebuilds the table from its own
// stored definition and copies every column by name. The rebuild drops the table's indexes, so
// they are recreated from the definitions saved beforehand.
func Migrate(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ApiToken{}) {
		return nil
	}
	if db.Name() == "postgres" {
		return db.Exec("ALTER TABLE api_tokens ALTER COLUMN scope DROP DEFAULT").Error
	}

	var indexes []string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'api_tokens' AND sql IS NOT NULL").
		Scan(&indexes).Error; err != nil {
		return fmt.Errorf("read api_tokens indexes: %w", err)
	}
	if err := db.Migrator().AlterColumn(&ApiToken{}, "Scope"); err != nil {
		return fmt.Errorf("drop api_tokens.scope default: %w", err)
	}
	for _, stmt := range indexes {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("recreate api_tokens index: %w", err)
		}
	}
	return nil
}
