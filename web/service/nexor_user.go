package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexor/panel/database"
	"github.com/nexor/panel/database/model"
	"gorm.io/gorm"
)

// NexorUserService synchronizes Nexor VPN users (table users) with Xray inbound clients.
type NexorUserService struct {
	inb InboundService
	set SettingService
}

func bytesToTotalGB(b int64) int64 {
	if b <= 0 {
		return 0
	}
	gb := b / (1024 * 1024 * 1024)
	if gb < 1 {
		return 1
	}
	return gb
}

func (s *NexorUserService) defaultFlow(in *model.Inbound) string {
	clients, err := s.inb.GetClients(in)
	if err != nil || len(clients) == 0 {
		if in.Protocol == model.VLESS {
			return "xtls-rprx-vision"
		}
		return ""
	}
	return clients[0].Flow
}

func (s *NexorUserService) buildNewClient(in *model.Inbound, subID string, expireMs int64, trafficBytes int64, deviceLimit int, enabled bool) (model.Client, error) {
	now := time.Now().UnixMilli()
	cl := model.Client{
		Email:      strings.ToLower(subID) + "@nexor.local",
		SubID:      subID,
		ExpiryTime: expireMs,
		TotalGB:    bytesToTotalGB(trafficBytes),
		LimitIP:    deviceLimit,
		Enable:     enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	switch in.Protocol {
	case model.VLESS, model.VMESS:
		cl.ID = uuid.NewString()
		cl.Flow = s.defaultFlow(in)
		if cl.Flow == "" && in.Protocol == model.VLESS {
			cl.Flow = "xtls-rprx-vision"
		}
	case model.Trojan:
		cl.Password = strings.ReplaceAll(uuid.NewString(), "-", "")
	case model.Shadowsocks:
		cl.ID = uuid.NewString()
		cl.Password = strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	default:
		return cl, fmt.Errorf("unsupported inbound protocol for Nexor user: %s", in.Protocol)
	}
	return cl, nil
}

func (s *NexorUserService) buildConfigURL(subUUID string) (string, error) {
	base, err := s.set.GetSubURI()
	if err != nil {
		return "", err
	}
	path, err := s.set.GetSubPath()
	if err != nil {
		return "", err
	}
	base = strings.TrimRight(base, "/")
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/sub/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	return fmt.Sprintf("%s%s/%s", base, path, subUUID), nil
}

// CreateVpnUser adds a DB row and an Xray client on the given inbound. Returns whether Xray needs a full restart.
func (s *NexorUserService) CreateVpnUser(username string, inboundId int, expireMs int64, trafficLimitBytes int64, deviceLimit int) (*model.VpnUser, *model.Subscription, bool, error) {
	if strings.TrimSpace(username) == "" {
		return nil, nil, false, errors.New("username is required")
	}
	inbound, err := s.inb.GetInbound(inboundId)
	if err != nil {
		return nil, nil, false, err
	}
	subID := uuid.NewString()
	cl, err := s.buildNewClient(inbound, subID, expireMs, trafficLimitBytes, deviceLimit, true)
	if err != nil {
		return nil, nil, false, err
	}
	key := s.inb.GetClientPrimaryKey(inbound.Protocol, cl)

	db := database.GetDB()
	u := &model.VpnUser{
		Username:      strings.TrimSpace(username),
		InboundId:     inboundId,
		ClientEmail:   cl.Email,
		XrayClientKey: key,
		ExpireDate:    expireMs,
		TrafficLimit:  trafficLimitBytes,
		DeviceLimit:   deviceLimit,
		Status:        model.VpnUserStatusActive,
	}
	if err := db.Create(u).Error; err != nil {
		return nil, nil, false, err
	}
	cfgURL, err := s.buildConfigURL(subID)
	if err != nil {
		db.Delete(u)
		return nil, nil, false, err
	}
	sub := &model.Subscription{
		UserId:    u.Id,
		UUID:      subID,
		ConfigURL: cfgURL,
	}
	if err := db.Create(sub).Error; err != nil {
		db.Delete(u)
		return nil, nil, false, err
	}

	settingsJSON, err := json.Marshal(map[string][]model.Client{"clients": {cl}})
	if err != nil {
		db.Delete(sub)
		db.Delete(u)
		return nil, nil, false, err
	}
	needRestart, err := s.inb.AddInboundClient(&model.Inbound{
		Id:       inboundId,
		Settings: string(settingsJSON),
	})
	if err != nil {
		db.Delete(sub)
		db.Delete(u)
		return nil, nil, false, err
	}
	return u, sub, needRestart, nil
}

// ListVpnUsers returns all VPN users ordered by id.
func (s *NexorUserService) ListVpnUsers() ([]model.VpnUser, error) {
	db := database.GetDB()
	var list []model.VpnUser
	err := db.Model(model.VpnUser{}).Order("id asc").Find(&list).Error
	return list, err
}

// GetVpnUser returns one VPN user by primary key.
func (s *NexorUserService) GetVpnUser(id int) (*model.VpnUser, error) {
	db := database.GetDB()
	var u model.VpnUser
	err := db.First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetSubscription returns the subscription row for a user.
func (s *NexorUserService) GetSubscription(userId int) (*model.Subscription, error) {
	db := database.GetDB()
	var sub model.Subscription
	err := db.Where("user_id = ?", userId).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// UpdateVpnUser updates limits and expiry on the Nexor row and syncs the inbound client.
func (s *NexorUserService) UpdateVpnUser(id int, username *string, expireMs *int64, trafficLimit *int64, deviceLimit *int) (bool, error) {
	u, err := s.GetVpnUser(id)
	if err != nil {
		return false, err
	}
	inbound, err := s.inb.GetInbound(u.InboundId)
	if err != nil {
		return false, err
	}
	clients, err := s.inb.GetClients(inbound)
	if err != nil {
		return false, err
	}
	var cl *model.Client
	for i := range clients {
		if strings.EqualFold(clients[i].Email, u.ClientEmail) {
			cl = &clients[i]
			break
		}
	}
	if cl == nil {
		return false, errors.New("client not found on inbound")
	}
	if username != nil {
		u.Username = strings.TrimSpace(*username)
	}
	if expireMs != nil {
		u.ExpireDate = *expireMs
		cl.ExpiryTime = *expireMs
	}
	if trafficLimit != nil {
		u.TrafficLimit = *trafficLimit
		cl.TotalGB = bytesToTotalGB(*trafficLimit)
	}
	if deviceLimit != nil {
		u.DeviceLimit = *deviceLimit
		cl.LimitIP = *deviceLimit
	}
	cl.UpdatedAt = time.Now().UnixMilli()
	if u.Status == model.VpnUserStatusDisabled {
		cl.Enable = false
	} else {
		cl.Enable = true
	}

	db := database.GetDB()
	if err := db.Save(u).Error; err != nil {
		return false, err
	}

	settingsJSON, err := json.Marshal(map[string][]model.Client{"clients": {*cl}})
	if err != nil {
		return false, err
	}
	return s.inb.UpdateInboundClient(&model.Inbound{
		Id:       u.InboundId,
		Settings: string(settingsJSON),
	}, u.XrayClientKey)
}

// DeleteVpnUser removes the client from Xray and deletes Nexor rows.
func (s *NexorUserService) DeleteVpnUser(id int) (bool, error) {
	u, err := s.GetVpnUser(id)
	if err != nil {
		return false, err
	}
	needRestart, err := s.inb.DelInboundClient(u.InboundId, u.XrayClientKey)
	if err != nil {
		return false, err
	}
	db := database.GetDB()
	_ = db.Where("user_id = ?", u.Id).Delete(&model.TrafficRecord{}).Error
	_ = db.Where("user_id = ?", u.Id).Delete(&model.UserSession{}).Error
	_ = db.Where("user_id = ?", u.Id).Delete(&model.Subscription{}).Error
	if err := db.Delete(u).Error; err != nil {
		return needRestart, err
	}
	return needRestart, nil
}

// SetVpnUserEnabled toggles the client in Xray and Nexor status.
func (s *NexorUserService) SetVpnUserEnabled(id int, enabled bool) (bool, error) {
	u, err := s.GetVpnUser(id)
	if err != nil {
		return false, err
	}
	if enabled {
		u.Status = model.VpnUserStatusActive
	} else {
		u.Status = model.VpnUserStatusDisabled
	}
	inbound, err := s.inb.GetInbound(u.InboundId)
	if err != nil {
		return false, err
	}
	clients, err := s.inb.GetClients(inbound)
	if err != nil {
		return false, err
	}
	var cl *model.Client
	for i := range clients {
		if strings.EqualFold(clients[i].Email, u.ClientEmail) {
			cl = &clients[i]
			break
		}
	}
	if cl == nil {
		return false, errors.New("client not found on inbound")
	}
	cl.Enable = enabled
	cl.UpdatedAt = time.Now().UnixMilli()
	db := database.GetDB()
	if err := db.Model(u).Update("status", u.Status).Error; err != nil {
		return false, err
	}
	settingsJSON, err := json.Marshal(map[string][]model.Client{"clients": {*cl}})
	if err != nil {
		return false, err
	}
	return s.inb.UpdateInboundClient(&model.Inbound{
		Id:       u.InboundId,
		Settings: string(settingsJSON),
	}, u.XrayClientKey)
}

// ExtendVpnUser adds delta to expiry time (milliseconds duration).
func (s *NexorUserService) ExtendVpnUser(id int, extendMs int64) (bool, error) {
	u, err := s.GetVpnUser(id)
	if err != nil {
		return false, err
	}
	newExp := u.ExpireDate + extendMs
	if newExp < time.Now().UnixMilli() {
		newExp = time.Now().UnixMilli() + extendMs
	}
	return s.UpdateVpnUser(id, nil, &newExp, nil, nil)
}

// TouchSession records or updates an IP session for device limiting (used by jobs / future hooks).
func (s *NexorUserService) TouchSession(userId int, ip string, nowMs int64) error {
	if ip == "" {
		return nil
	}
	db := database.GetDB()
	var sess model.UserSession
	err := db.Where("user_id = ? AND ip = ?", userId, ip).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&model.UserSession{
			UserId:   userId,
			Ip:       ip,
			LastSeen: nowMs,
		}).Error
	}
	if err != nil {
		return err
	}
	sess.LastSeen = nowMs
	return db.Save(&sess).Error
}

// RecordTraffic appends a traffic rollup row.
func (s *NexorUserService) RecordTraffic(userId int, up, down int64, atMs int64) error {
	db := database.GetDB()
	return db.Create(&model.TrafficRecord{
		UserId:     userId,
		BytesUp:    up,
		BytesDown:  down,
		RecordedAt: atMs,
	}).Error
}
