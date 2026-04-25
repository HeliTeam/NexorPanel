// Package database provides database initialization, migration, and management utilities
// for the Nexor panel using GORM.
package database

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/nexor/panel/config"
	"github.com/nexor/panel/database/model"
	"github.com/nexor/panel/util/crypto"
	"github.com/nexor/panel/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func initModels() error {
	models := []any{
		&model.Admin{},
		&model.VpnUser{},
		&model.Subscription{},
		&model.TrafficRecord{},
		&model.UserSession{},
		&model.APIKey{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.CustomGeoResource{},
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func looksLikeBcrypt(s string) bool {
	if len(s) < 4 {
		return false
	}
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
}

// upgradeLegacyUserPasswords converts plaintext passwords in the legacy `users` table to bcrypt.
func upgradeLegacyUserPasswords() error {
	if !db.Migrator().HasTable("users") {
		return nil
	}
	if db.Migrator().HasColumn("users", "client_email") {
		return nil
	}
	var rows []struct {
		Id       int
		Password string
	}
	if err := db.Table("users").Select("id", "password").Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if looksLikeBcrypt(r.Password) {
			continue
		}
		hashed, err := crypto.HashPasswordAsBcrypt(r.Password)
		if err != nil {
			return err
		}
		if err := db.Table("users").Where("id = ?", r.Id).Update("password", hashed).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateLegacyPanelUsersToAdmins copies the old panel `users` table into `admins` and drops `users`.
func migrateLegacyPanelUsersToAdmins() error {
	var adminCount int64
	if err := db.Model(&model.Admin{}).Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}
	if !db.Migrator().HasTable("users") {
		return nil
	}
	if db.Migrator().HasColumn("users", "client_email") {
		return nil
	}
	if err := upgradeLegacyUserPasswords(); err != nil {
		return fmt.Errorf("upgrade legacy passwords: %w", err)
	}
	if err := db.Exec(`
		INSERT INTO admins (id, nickname, password_hash, created_at)
		SELECT id, username, password, 0 FROM users
	`).Error; err != nil {
		return fmt.Errorf("migrate users to admins: %w", err)
	}
	if err := db.Migrator().DropTable("users"); err != nil {
		return fmt.Errorf("drop legacy users: %w", err)
	}
	if db.Dialector.Name() == "sqlite" {
		var maxID int64
		if err := db.Model(&model.Admin{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
			return err
		}
		if maxID > 0 {
			_ = db.Exec("DELETE FROM sqlite_sequence WHERE name = ?", "admins").Error
			if err := db.Exec("INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)", "admins", maxID).Error; err != nil {
				return err
			}
		}
	}
	if db.Dialector.Name() == "postgres" {
		_ = db.Exec(`SELECT setval(pg_get_serial_sequence('admins','id'), COALESCE((SELECT MAX(id) FROM admins), 1))`).Error
	}
	return nil
}

// runSeeders migrates admin passwords to bcrypt and records seeder execution to prevent re-running.
func runSeeders(isAdminsEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking history_of_seeders: %v", err)
		return err
	}

	if empty && isAdminsEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	}

	var seedersHistory []string
	db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

	if !slices.Contains(seedersHistory, "UserPasswordHash") && !isAdminsEmpty {
		var admins []model.Admin
		db.Find(&admins)

		for _, admin := range admins {
			if looksLikeBcrypt(admin.PasswordHash) {
				continue
			}
			hashedPassword, err := crypto.HashPasswordAsBcrypt(admin.PasswordHash)
			if err != nil {
				log.Printf("Error hashing password for admin '%s': %v", admin.Nickname, err)
				return err
			}
			db.Model(&admin).Update("password_hash", hashedPassword)
		}

		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	}

	return nil
}

// isTableEmpty returns true if the named table contains zero rows.
func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

func databaseURL() string {
	if u := os.Getenv("NEXOR_DATABASE_URL"); u != "" {
		return u
	}
	return os.Getenv("DATABASE_URL")
}

// InitDB sets up the database connection, migrates models, and runs seeders.
// When NEXOR_DATABASE_URL or DATABASE_URL is set, PostgreSQL is used and dbPath is ignored.
func InitDB(dbPath string) error {
	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}

	var err error
	if dsn := databaseURL(); dsn != "" {
		db, err = gorm.Open(postgres.Open(dsn), c)
	} else {
		dir := path.Dir(dbPath)
		err = os.MkdirAll(dir, fs.ModePerm)
		if err != nil {
			return err
		}
		db, err = gorm.Open(sqlite.Open(dbPath), c)
	}
	if err != nil {
		return err
	}

	if err := initModels(); err != nil {
		return err
	}

	if err := migrateLegacyPanelUsersToAdmins(); err != nil {
		return err
	}

	isAdminsEmpty, err := isTableEmpty("admins")
	if err != nil {
		return err
	}

	return runSeeders(isAdminsEmpty)
}

// CloseDB closes the database connection if it exists.
func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsSQLiteDB checks if the given file is a valid SQLite database by reading its signature.
func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// Checkpoint performs a WAL checkpoint on the SQLite database to ensure data consistency.
func Checkpoint() error {
	if db == nil || db.Dialector.Name() != "sqlite" {
		return nil
	}
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
