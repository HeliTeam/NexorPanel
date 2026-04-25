package job

import (
	"time"

	"github.com/nexor/panel/database"
	"github.com/nexor/panel/database/model"
	"github.com/nexor/panel/logger"
	"github.com/nexor/panel/web/service"
)

const (
	nexorInactiveDays     = 90
	nexorSessionPruneDays = 30
)

// NexorVpnCronJob auto-disables expired or over-quota VPN users and prunes stale session rows.
type NexorVpnCronJob struct {
	vpn     service.NexorUserService
	inbound service.InboundService
	xray    service.XrayService
}

// NewNexorVpnCronJob builds a cron job with zero-value service receivers (same pattern as other jobs).
func NewNexorVpnCronJob() *NexorVpnCronJob {
	return new(NexorVpnCronJob)
}

// Run applies Nexor automation policies.
func (j *NexorVpnCronJob) Run() {
	nowMs := time.Now().UnixMilli()
	list, err := j.vpn.ListVpnUsers()
	if err != nil {
		logger.Warning("nexor vpn cron: list users:", err)
		return
	}
	for i := range list {
		u := &list[i]
		if u.Status != model.VpnUserStatusActive {
			continue
		}
		if u.ExpireDate > 0 && u.ExpireDate < nowMs {
			needRestart, err := j.vpn.SetVpnUserEnabled(u.Id, false)
			if err != nil {
				logger.Warning("nexor vpn cron: disable expired:", u.Id, err)
				continue
			}
			if needRestart {
				j.xray.SetToNeedRestart()
			}
			logger.Infof("nexor vpn cron: disabled expired user id=%d", u.Id)
			continue
		}
		if u.TrafficLimit > 0 {
			ct, err := j.inbound.GetClientTrafficByEmail(u.ClientEmail)
			if err != nil || ct == nil {
				continue
			}
			used := ct.Up + ct.Down
			if used >= u.TrafficLimit {
				needRestart, err := j.vpn.SetVpnUserEnabled(u.Id, false)
				if err != nil {
					logger.Warning("nexor vpn cron: disable over quota:", u.Id, err)
					continue
				}
				if needRestart {
					j.xray.SetToNeedRestart()
				}
				logger.Infof("nexor vpn cron: disabled over-traffic user id=%d", u.Id)
			}
		}
	}

	j.cleanupInactiveUsers(nowMs)
	j.pruneOldSessions()
}

func (j *NexorVpnCronJob) cleanupInactiveUsers(nowMs int64) {
	cutoff := nowMs - int64(nexorInactiveDays)*24*3600*1000
	list, err := j.vpn.ListVpnUsers()
	if err != nil {
		return
	}
	for i := range list {
		u := &list[i]
		ct, err := j.inbound.GetClientTrafficByEmail(u.ClientEmail)
		if err != nil || ct == nil {
			continue
		}
		if ct.LastOnline > cutoff {
			continue
		}
		if ct.Up+ct.Down > 0 {
			continue
		}
		needRestart, err := j.vpn.DeleteVpnUser(u.Id)
		if err != nil {
			logger.Warning("nexor vpn cron: delete inactive:", u.Id, err)
			continue
		}
		if needRestart {
			j.xray.SetToNeedRestart()
		}
		logger.Infof("nexor vpn cron: removed inactive user id=%d", u.Id)
	}
}

func (j *NexorVpnCronJob) pruneOldSessions() {
	db := database.GetDB()
	if db == nil {
		return
	}
	cut := time.Now().UnixMilli() - int64(nexorSessionPruneDays)*24*3600*1000
	_ = db.Where("last_seen < ?", cut).Delete(&model.UserSession{}).Error
}
