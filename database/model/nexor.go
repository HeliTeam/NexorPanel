// Package model — Nexor VPN user domain tables (table "users" is VPN subscribers, not panel admins).
package model

// VpnUser is a commercial VPN subscriber linked to one inbound client row in Xray settings.
type VpnUser struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string `json:"username" gorm:"size:128;not null;index"`
	InboundId    int    `json:"inboundId" gorm:"not null;index"`
	ClientEmail  string `json:"clientEmail" gorm:"size:255;not null;uniqueIndex"`
	XrayClientKey string `json:"-" gorm:"size:255;not null;column:xray_client_key"` // protocol-specific id for DelInboundClient
	ExpireDate   int64  `json:"expireDate" gorm:"not null;index:idx_vpn_users_status_expire,priority:2"` // unix milliseconds
	TrafficLimit int64  `json:"trafficLimit" gorm:"not null"`                       // bytes; 0 = unlimited
	DeviceLimit  int    `json:"deviceLimit" gorm:"not null"`
	Status       string `json:"status" gorm:"size:32;not null;default:active;index:idx_vpn_users_status_expire,priority:1"`
	CreatedAt    int64  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt    int64  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (VpnUser) TableName() string {
	return "users"
}

const (
	VpnUserStatusActive   = "active"
	VpnUserStatusDisabled = "disabled"
)

// Subscription stores subscription UUID and public config URL for a VPN user.
type Subscription struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"userId" gorm:"not null;index"`
	UUID      string `json:"uuid" gorm:"size:36;not null;uniqueIndex"`
	ConfigURL string `json:"configUrl" gorm:"not null"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}

// TrafficRecord is an optional rollup of per-user traffic (in addition to xray client stats).
type TrafficRecord struct {
	Id         int   `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId     int   `json:"userId" gorm:"not null;index:idx_traffic_user_recorded,priority:1"`
	BytesUp    int64 `json:"bytesUp"`
	BytesDown  int64 `json:"bytesDown"`
	RecordedAt int64 `json:"recordedAt" gorm:"not null;index:idx_traffic_user_recorded,priority:2"`
}

func (TrafficRecord) TableName() string {
	return "traffic"
}

// UserSession tracks active client IPs with last-seen time (device_limit / analytics).
type UserSession struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId   int    `json:"userId" gorm:"not null;uniqueIndex:idx_session_user_ip,priority:1"`
	Ip       string `json:"ip" gorm:"size:64;not null;uniqueIndex:idx_session_user_ip,priority:2"`
	LastSeen int64  `json:"lastSeen" gorm:"not null;index"`
}

func (UserSession) TableName() string {
	return "sessions"
}

// APIKey stores a hashed API key for REST/bot access.
type APIKey struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	KeyHash     string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Permissions string `json:"permissions" gorm:"type:text"` // JSON array of scope strings
	CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime"`
	LastUsedAt  int64  `json:"lastUsedAt" gorm:"default:0"`
}

func (APIKey) TableName() string {
	return "api_keys"
}
