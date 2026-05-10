package version

// 通过 -ldflags "-X" 在编译期注入，默认值用于 go run / 直接 go build。
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// Info 汇总版本信息，便于日志/接口输出
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}
