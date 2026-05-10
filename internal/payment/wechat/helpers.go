package wechat

import (
	"bytes"

	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	wxdownloader "github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
)

// bytesReader 把 []byte 包成可重复读取的 io.Reader
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

// buildVerifier 复用 client 自动注册到全局 MgrInstance 的证书下载器，
// 构造一个用于回调验签的 SHA256WithRSAVerifier。
func buildVerifier(mchID string) *verifiers.SHA256WithRSAVerifier {
	mgr := wxdownloader.MgrInstance()
	visitor := mgr.GetCertificateVisitor(mchID)
	return verifiers.NewSHA256WithRSAVerifier(visitor)
}
