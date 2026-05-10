package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Log    LogConfig    `mapstructure:"log"`
	Auth   AuthConfig   `mapstructure:"auth"`
	Notify NotifyConfig `mapstructure:"notify"`
	WeChat WeChatConfig `mapstructure:"wechat"`
	Alipay AlipayConfig `mapstructure:"alipay"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type MySQLConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	Charset         string        `mapstructure:"charset"`
	ParseTime       bool          `mapstructure:"parse_time"`
	Loc             string        `mapstructure:"loc"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN 返回 GORM/database-sql 用的 MySQL 连接串
func (m MySQLConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		m.User, m.Password, m.Host, m.Port, m.Database,
		m.Charset, m.ParseTime, m.Loc,
	)
}

type LogConfig struct {
	Level       string   `mapstructure:"level"`
	Encoding    string   `mapstructure:"encoding"`
	OutputPaths []string `mapstructure:"output_paths"`
	FilePath    string   `mapstructure:"file_path"`
	MaxSizeMB   int      `mapstructure:"max_size_mb"`
	MaxBackups  int      `mapstructure:"max_backups"`
	MaxAgeDays  int      `mapstructure:"max_age_days"`
	Compress    bool     `mapstructure:"compress"`
}

type AuthConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Header  string   `mapstructure:"header"`
	APIKeys []string `mapstructure:"api_keys"`
}

type NotifyConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type WeChatConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	AppID            string `mapstructure:"app_id"`
	MchID            string `mapstructure:"mch_id"`
	SerialNo         string `mapstructure:"serial_no"`
	APIv3Key         string `mapstructure:"api_v3_key"`
	PrivateKeyPath   string `mapstructure:"private_key_path"`
	NotifyPath       string `mapstructure:"notify_path"`
	RefundNotifyPath string `mapstructure:"refund_notify_path"`
}

type AlipayConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	AppID            string `mapstructure:"app_id"`
	PrivateKey       string `mapstructure:"private_key"`
	AliPublicKey     string `mapstructure:"ali_public_key"`
	IsProduction     bool   `mapstructure:"is_production"`
	NotifyPath       string `mapstructure:"notify_path"`
	RefundNotifyPath string `mapstructure:"refund_notify_path"`
	ReturnURL        string `mapstructure:"return_url"`
}

// Load 从指定路径加载配置文件，并支持环境变量覆盖（前缀 NEXPAY_，分隔符 __）
// 例如 NEXPAY_MYSQL__PASSWORD 会覆盖 mysql.password
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("NEXPAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
