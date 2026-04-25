package service

import (
	"errors"
	"strings"

	"github.com/nexor/panel/database"
	"github.com/nexor/panel/database/model"
	"github.com/nexor/panel/logger"
	"github.com/nexor/panel/util/crypto"
	ldaputil "github.com/nexor/panel/util/ldap"
	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

// AdminService provides authentication and credential updates for panel administrators.
type AdminService struct {
	settingService SettingService
}

// GetFirstAdmin returns the first admin row (legacy single-admin setups).
func (s *AdminService) GetFirstAdmin() (*model.Admin, error) {
	db := database.GetDB()
	admin := &model.Admin{}
	err := db.Model(model.Admin{}).First(admin).Error
	if err != nil {
		return nil, err
	}
	return admin, nil
}

// CheckAdmin validates nickname + password (+ optional 2FA) and returns the admin on success.
func (s *AdminService) CheckAdmin(nickname string, password string, twoFactorCode string) (*model.Admin, error) {
	db := database.GetDB()
	admin := &model.Admin{}
	err := db.Model(model.Admin{}).Where("nickname = ?", nickname).First(admin).Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.New("invalid credentials")
	} else if err != nil {
		logger.Warning("check admin err:", err)
		return nil, err
	}

	if !crypto.CheckPasswordHash(admin.PasswordHash, password) {
		ldapEnabled, _ := s.settingService.GetLdapEnable()
		if !ldapEnabled {
			return nil, errors.New("invalid credentials")
		}

		host, _ := s.settingService.GetLdapHost()
		port, _ := s.settingService.GetLdapPort()
		useTLS, _ := s.settingService.GetLdapUseTLS()
		bindDN, _ := s.settingService.GetLdapBindDN()
		ldapPass, _ := s.settingService.GetLdapPassword()
		baseDN, _ := s.settingService.GetLdapBaseDN()
		userFilter, _ := s.settingService.GetLdapUserFilter()
		userAttr, _ := s.settingService.GetLdapUserAttr()

		cfg := ldaputil.Config{
			Host:       host,
			Port:       port,
			UseTLS:     useTLS,
			BindDN:     bindDN,
			Password:   ldapPass,
			BaseDN:     baseDN,
			UserFilter: userFilter,
			UserAttr:   userAttr,
		}
		ok, err := ldaputil.AuthenticateUser(cfg, nickname, password)
		if err != nil || !ok {
			return nil, errors.New("invalid credentials")
		}
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		logger.Warning("check two factor err:", err)
		return nil, err
	}

	if twoFactorEnable {
		twoFactorToken, err := s.settingService.GetTwoFactorToken()
		if err != nil {
			logger.Warning("check two factor token err:", err)
			return nil, err
		}
		if gotp.NewDefaultTOTP(twoFactorToken).Now() != twoFactorCode {
			return nil, errors.New("invalid 2fa code")
		}
	}

	return admin, nil
}

// UpdateAdmin updates nickname and password for the admin with the given id.
func (s *AdminService) UpdateAdmin(id int, nickname string, password string) error {
	db := database.GetDB()
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return err
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		return err
	}

	if twoFactorEnable {
		s.settingService.SetTwoFactorEnable(false)
		s.settingService.SetTwoFactorToken("")
	}

	return db.Model(model.Admin{}).
		Where("id = ?", id).
		Updates(map[string]any{"nickname": nickname, "password_hash": hashedPassword}).
		Error
}

// UpdateFirstAdmin updates the first admin or creates one if none exist (CLI / recovery).
func (s *AdminService) UpdateFirstAdmin(nickname string, password string) error {
	if strings.TrimSpace(nickname) == "" {
		return errors.New("nickname can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)
	if er != nil {
		return er
	}

	db := database.GetDB()
	admin := &model.Admin{}
	err := db.Model(model.Admin{}).First(admin).Error
	if database.IsNotFound(err) {
		admin.Nickname = nickname
		admin.PasswordHash = hashedPassword
		return db.Model(model.Admin{}).Create(admin).Error
	} else if err != nil {
		return err
	}
	admin.Nickname = nickname
	admin.PasswordHash = hashedPassword
	return db.Save(admin).Error
}

// CreateAdmin inserts a new administrator (CLI). Fails if nickname is already taken.
func (s *AdminService) CreateAdmin(nickname string, password string) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return errors.New("nickname can not be empty")
	}
	if password == "" {
		return errors.New("password can not be empty")
	}
	hashed, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(model.Admin{}).Where("nickname = ?", nickname).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("admin with this nickname already exists")
	}
	return db.Create(&model.Admin{Nickname: nickname, PasswordHash: hashed}).Error
}
