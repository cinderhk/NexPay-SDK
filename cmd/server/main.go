package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/nexpay/nexpay-sdk/internal/api"
	"github.com/nexpay/nexpay-sdk/internal/config"
	"github.com/nexpay/nexpay-sdk/internal/model"
	"github.com/nexpay/nexpay-sdk/internal/payment"
	"github.com/nexpay/nexpay-sdk/internal/payment/alipay"
	"github.com/nexpay/nexpay-sdk/internal/payment/wechat"
	"github.com/nexpay/nexpay-sdk/internal/repository"
	"github.com/nexpay/nexpay-sdk/internal/service"
	"github.com/nexpay/nexpay-sdk/internal/version"
	"github.com/nexpay/nexpay-sdk/pkg/logger"
	"go.uber.org/zap"
)

const usageText = `nexpay - 统一支付服务

Usage:
  nexpay [flags]               启动 HTTP 服务（默认）
  nexpay migrate [flags]       执行数据库迁移（GORM AutoMigrate）后退出
  nexpay version               打印版本信息

Flags:
  --config string              配置文件路径 (default "configs/config.yaml")
`

func main() {
	args := os.Args[1:]

	// 默认行为：不带参数时直接启动 HTTP 服务
	if len(args) == 0 {
		runServe(args)
		return
	}

	cmd := args[0]
	args = args[1:]

	switch cmd {
	case "migrate", "-m", "--migrate":
		runMigrate(args)
	case "version", "-v", "--version":
		printVersion()
	case "help", "-h", "--help":
		fmt.Print(usageText)
	default:
		// 兼容历史行为：未知短横线参数仍按 serve 模式处理
		if len(cmd) > 0 && cmd[0] == '-' {
			runServe(append([]string{cmd}, args...))
			return
		}
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usageText)
		os.Exit(2)
	}
}

func parseFlags(name string, args []string) string {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cfgPath := fs.String("config", "configs/config.yaml", "config file path")
	_ = fs.Parse(args)
	return *cfgPath
}

func loadCtx(name string, args []string) (*config.Config, *zap.Logger) {
	cfgPath := parseFlags(name, args)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(
		cfg.Log.Level,
		cfg.Log.Encoding,
		cfg.Log.OutputPaths,
		logger.FileRotateOptions{
			Path:       cfg.Log.FilePath,
			MaxSizeMB:  cfg.Log.MaxSizeMB,
			MaxBackups: cfg.Log.MaxBackups,
			MaxAgeDays: cfg.Log.MaxAgeDays,
			Compress:   cfg.Log.Compress,
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}

	v := version.Get()
	log.Info("nexpay starting",
		zap.String("cmd", name),
		zap.String("version", v.Version),
		zap.String("commit", v.Commit),
		zap.String("build_time", v.BuildTime),
	)
	return cfg, log
}

func printVersion() {
	b, _ := json.MarshalIndent(version.Get(), "", "  ")
	fmt.Println(string(b))
}

// runMigrate 仅执行数据库迁移，跑完即退出
func runMigrate(args []string) {
	cfg, log := loadCtx("migrate", args)
	defer log.Sync() //nolint:errcheck

	db, err := repository.NewDB(cfg.MySQL)
	if err != nil {
		log.Fatal("init mysql failed", zap.Error(err))
	}
	log.Info("mysql connected", zap.String("dsn", maskDSN(cfg.MySQL.DSN())))

	if err := repository.Migrate(db); err != nil {
		log.Fatal("migrate failed", zap.Error(err))
	}
	log.Info("migrate done")
}

func runServe(args []string) {
	cfg, log := loadCtx("serve", args)
	defer log.Sync() //nolint:errcheck

	gin.SetMode(cfg.Server.Mode)

	db, err := repository.NewDB(cfg.MySQL)
	if err != nil {
		log.Fatal("init mysql failed", zap.Error(err))
	}
	log.Info("mysql connected", zap.String("dsn", maskDSN(cfg.MySQL.DSN())))

	orderRepo := repository.NewOrderRepository(db)
	refundRepo := repository.NewRefundRepository(db)
	notifyRepo := repository.NewNotifyLogRepository(db)

	providers := buildProviders(context.Background(), cfg, log)
	if len(providers) == 0 {
		log.Warn("no payment channel enabled, server will start in degraded mode")
	}

	svc := service.NewPaymentService(providers, orderRepo, refundRepo, notifyRepo, log)
	handler := api.NewHandler(svc, log)
	router := api.NewRouter(handler, log, cfg.Auth)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info("server started", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", zap.Error(err))
	}
	log.Info("bye")
}

func buildProviders(ctx context.Context, cfg *config.Config, log *zap.Logger) map[model.Channel]payment.Provider {
	providers := make(map[model.Channel]payment.Provider)

	if cfg.WeChat.Enabled {
		p, err := wechat.New(ctx, cfg.WeChat, cfg.Notify.BaseURL)
		if err != nil {
			log.Error("init wechat provider failed", zap.Error(err))
		} else {
			providers[model.ChannelWeChat] = p
			log.Info("wechat provider enabled")
		}
	}

	if cfg.Alipay.Enabled {
		p, err := alipay.New(cfg.Alipay, cfg.Notify.BaseURL)
		if err != nil {
			log.Error("init alipay provider failed", zap.Error(err))
		} else {
			providers[model.ChannelAlipay] = p
			log.Info("alipay provider enabled")
		}
	}

	return providers
}

// maskDSN 在日志里隐藏密码
func maskDSN(dsn string) string {
	idx := -1
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == ':' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return dsn
	}
	end := -1
	for i := idx + 1; i < len(dsn); i++ {
		if dsn[i] == '@' {
			end = i
			break
		}
	}
	if end < 0 {
		return dsn
	}
	return dsn[:idx+1] + "***" + dsn[end:]
}
