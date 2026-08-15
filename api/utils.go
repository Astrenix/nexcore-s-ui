package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

// getRemoteIp 只在直连对端是可信反代(回环/私网)时才采信 X-Forwarded-For。
// 详见 common.ClientIP —— 这是登录爆破节流的 key 来源,伪造 XFF 会让节流失效。
func getRemoteIp(c *gin.Context) string {
	return common.ClientIP(c.Request.RemoteAddr, c.GetHeader("X-Forwarded-For"))
}

func getHostname(c *gin.Context) string {
	// 客户端分享链接的 add 字段就是这个 hostname。优先级:
	//   1) panel settings.webDomain — 管理员显式设的"对外域名",DNS 必通
	//      (面板自身 SSL 流程已经验过解析),最稳
	//   2) c.Request.Host — 用户访问面板的 Host header,作为 fallback
	// 旧版只看 (2),如果用户用 IP 访问面板,生成的 vmess add 就全是 IP;
	// 即使 inbound TLS 用域名签的(server_name),add 写 IP 也会让追求"机场
	// 节点显示域名"的运维感到困惑;更糟的是 inbound TLS 用 wildcard 时
	// server_name 是 *.x.example,add 不能用 *,只能 fallback IP/Host。
	settingSvc := service.SettingService{}
	if domain, _ := settingSvc.GetWebDomain(); domain != "" {
		return domain
	}
	host := c.Request.Host
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(c.Request.Host)
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return host
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Msg = msg + ": " + err.Error()
		logger.Warning("failed :", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func checkLogin(c *gin.Context) {
	if !IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			pureJsonMsg(c, false, "Invalid login")
		} else {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
		}
		c.Abort()
	} else {
		c.Next()
	}
}
