package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexpay/nexpay-sdk/internal/config"
	"go.uber.org/zap"
)

// NewRouter 注册所有路由
func NewRouter(h *Handler, log *zap.Logger, authCfg config.AuthConfig) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), accessLog(log))

	r.GET("/healthz", h.Health)

	v1 := r.Group("/api/v1")
	{
		protected := v1.Group("")
		if authCfg.Enabled {
			protected.Use(apiKeyAuth(authCfg, log))
		}
		protected.POST("/payments", h.CreatePayment)
		protected.GET("/payments/:out_trade_no", h.QueryPayment)
		protected.POST("/refunds", h.Refund)

		v1.POST("/notify/wechat", h.WeChatNotify)
		v1.POST("/notify/alipay", h.AlipayNotify)
		v1.POST("/notify/wechat/refund", h.WeChatRefundNotify)
		v1.POST("/notify/alipay/refund", h.AlipayRefundNotify)
	}
	return r
}

func accessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("cost", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
