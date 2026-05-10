package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nexpay/nexpay-sdk/internal/config"
	"github.com/nexpay/nexpay-sdk/pkg/response"
	"go.uber.org/zap"
)

func apiKeyAuth(cfg config.AuthConfig, log *zap.Logger) gin.HandlerFunc {
	header := cfg.Header
	if header == "" {
		header = "X-API-Key"
	}

	allowed := make(map[string]struct{}, len(cfg.APIKeys))
	for _, k := range cfg.APIKeys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		allowed[k] = struct{}{}
	}

	return func(c *gin.Context) {
		if len(allowed) == 0 {
			log.Error("auth enabled but api_keys is empty")
			response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		token := c.GetHeader(header)
		if token == "" {
			authz := c.GetHeader("Authorization")
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token = strings.TrimSpace(authz[7:])
			}
		}
		if _, ok := allowed[token]; !ok {
			response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
