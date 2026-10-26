package middleware

import (
	"crypto/subtle"
	"net/http"

	"incus-web/config"
	"incus-web/model"

	"github.com/gin-gonic/gin"
)

// Auth 简单 Token 认证中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token != "" && len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// 常量时间比较，避免通过响应时间差逐字节猜测令牌
		if subtle.ConstantTimeCompare([]byte(token), []byte(config.APIToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.ApiResponse{
				Code:  401,
				Error: "unauthorized: invalid or missing token",
			})
			return
		}
		c.Next()
	}
}
