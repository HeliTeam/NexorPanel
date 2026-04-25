// Package rest exposes the public Nexor REST API (JWT or API key).
package rest

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nexor/panel/web/middleware"
	"github.com/nexor/panel/web/service"
)

const (
	scopeUsersRead    = "users:read"
	scopeUsersWrite   = "users:write"
	scopeAPIKeysWrite = "apikeys:write"
)

// RegisterRestAPI mounts /api routes on the root engine (not under panel base path).
func RegisterRestAPI(engine *gin.Engine, xraySvc *service.XrayService) {
	jwtSvc := &service.JWTAuthService{}
	keySvc := &service.APIKeyService{}
	vpnSvc := &service.NexorUserService{}
	adminSvc := &service.AdminService{}

	api := engine.Group("/api")
	api.Use(middleware.APIRateLimit())

	api.POST("/auth/login", func(c *gin.Context) { postAuthLogin(c, jwtSvc, adminSvc) })
	api.POST("/auth/refresh", func(c *gin.Context) { postAuthRefresh(c, jwtSvc) })

	auth := api.Group("")
	auth.Use(func(c *gin.Context) { restRequireAuth(c, jwtSvc, keySvc) })

	auth.POST("/auth/apikey", restRequireScope(scopeAPIKeysWrite), func(c *gin.Context) {
		postCreateAPIKey(c, keySvc)
	})

	auth.POST("/user/create", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		postUserCreate(c, vpnSvc, xraySvc)
	})
	auth.GET("/users", restRequireScope(scopeUsersRead), func(c *gin.Context) {
		getUsers(c, vpnSvc)
	})
	auth.GET("/user/:id", restRequireScope(scopeUsersRead), func(c *gin.Context) {
		getUser(c, vpnSvc)
	})
	auth.POST("/user/update", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		postUserUpdate(c, vpnSvc, xraySvc)
	})
	auth.DELETE("/user/:id", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		deleteUser(c, vpnSvc, xraySvc)
	})
	auth.GET("/user/:id/subscription", restRequireScope(scopeUsersRead), func(c *gin.Context) {
		getUserSubscription(c, vpnSvc)
	})
	auth.POST("/user/:id/enable", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		postUserEnable(c, vpnSvc, xraySvc, true)
	})
	auth.POST("/user/:id/disable", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		postUserEnable(c, vpnSvc, xraySvc, false)
	})
	auth.POST("/user/extend", restRequireScope(scopeUsersWrite), func(c *gin.Context) {
		postUserExtend(c, vpnSvc, xraySvc)
	})
}

func restRequireAuth(c *gin.Context, jwtSvc *service.JWTAuthService, keySvc *service.APIKeyService) {
	apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if apiKey != "" {
		row, err := keySvc.VerifyAPIKey(apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Set("auth_jwt", false)
		c.Set("auth_api_key_perms", row.Permissions)
		c.Next()
		return
	}
	h := c.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token or X-API-Key"})
		return
	}
	tok := strings.TrimSpace(h[7:])
	_, err := jwtSvc.ParseAccessToken(tok)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	c.Set("auth_jwt", true)
	c.Set("auth_api_key_perms", "")
	c.Next()
}

func restRequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("auth_jwt") {
			c.Next()
			return
		}
		perms, ok := c.Get("auth_api_key_perms")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if !service.ScopeAllows(perms.(string), scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
			return
		}
		c.Next()
	}
}

type loginBody struct {
	Nickname      string `json:"nickname"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"twoFactorCode"`
}

func postAuthLogin(c *gin.Context, jwtSvc *service.JWTAuthService, adminSvc *service.AdminService) {
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	admin, err := adminSvc.CheckAdmin(body.Nickname, body.Password, body.TwoFactorCode)
	if err != nil || admin == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	access, refresh, err := jwtSvc.IssuePair(admin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issue failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_in":    900,
	})
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func postAuthRefresh(c *gin.Context, jwtSvc *service.JWTAuthService) {
	var body refreshBody
	if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
		return
	}
	access, refresh, err := jwtSvc.RefreshPair(body.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_in":    900,
	})
}

type createAPIKeyBody struct {
	Permissions []string `json:"permissions"`
}

func postCreateAPIKey(c *gin.Context, keySvc *service.APIKeyService) {
	if !c.GetBool("auth_jwt") {
		c.JSON(http.StatusForbidden, gin.H{"error": "jwt required to mint api keys"})
		return
	}
	var body createAPIKeyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if len(body.Permissions) == 0 {
		body.Permissions = []string{"users:read", "users:write"}
	}
	plain, err := keySvc.CreateAPIKey(body.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"api_key": plain, "permissions": body.Permissions})
}

type createUserBody struct {
	Username     string `json:"username"`
	InboundID    int    `json:"inbound_id"`
	ExpireDate   int64  `json:"expire_date"`
	TrafficLimit int64  `json:"traffic_limit"`
	DeviceLimit  int    `json:"device_limit"`
}

func postUserCreate(c *gin.Context, vpn *service.NexorUserService, xraySvc *service.XrayService) {
	var body createUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	u, sub, needRestart, err := vpn.CreateVpnUser(body.Username, body.InboundID, body.ExpireDate, body.TrafficLimit, body.DeviceLimit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if needRestart && xraySvc != nil {
		xraySvc.SetToNeedRestart()
	}
	c.JSON(http.StatusCreated, gin.H{"user": u, "subscription": sub})
}

func getUsers(c *gin.Context, vpn *service.NexorUserService) {
	list, err := vpn.ListVpnUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": list})
}

func getUser(c *gin.Context, vpn *service.NexorUserService) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := vpn.GetVpnUser(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

type updateUserBody struct {
	ID           int     `json:"id"`
	Username     *string `json:"username"`
	ExpireDate   *int64  `json:"expire_date"`
	TrafficLimit *int64  `json:"traffic_limit"`
	DeviceLimit  *int    `json:"device_limit"`
}

func postUserUpdate(c *gin.Context, vpn *service.NexorUserService, xraySvc *service.XrayService) {
	var body updateUserBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	needRestart, err := vpn.UpdateVpnUser(body.ID, body.Username, body.ExpireDate, body.TrafficLimit, body.DeviceLimit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if needRestart && xraySvc != nil {
		xraySvc.SetToNeedRestart()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func deleteUser(c *gin.Context, vpn *service.NexorUserService, xraySvc *service.XrayService) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	needRestart, err := vpn.DeleteVpnUser(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if needRestart && xraySvc != nil {
		xraySvc.SetToNeedRestart()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func getUserSubscription(c *gin.Context, vpn *service.NexorUserService) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	sub, err := vpn.GetSubscription(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

func postUserEnable(c *gin.Context, vpn *service.NexorUserService, xraySvc *service.XrayService, enabled bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	needRestart, err := vpn.SetVpnUserEnabled(id, enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if needRestart && xraySvc != nil {
		xraySvc.SetToNeedRestart()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type extendBody struct {
	ID       int   `json:"id"`
	ExtendMs int64 `json:"extend_ms"`
}

func postUserExtend(c *gin.Context, vpn *service.NexorUserService, xraySvc *service.XrayService) {
	var body extendBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ID <= 0 || body.ExtendMs <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and extend_ms required"})
		return
	}
	needRestart, err := vpn.ExtendVpnUser(body.ID, body.ExtendMs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if needRestart && xraySvc != nil {
		xraySvc.SetToNeedRestart()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
