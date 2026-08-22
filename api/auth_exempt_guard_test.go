package api

// 鉴权豁免必须是【精确匹配 action】的白名单,不能靠子串/后缀。
//
// 2026-08-22 E571。原实现是 `strings.HasSuffix(c.Request.URL.Path, "login")`。
// 当前 53 个 action 里恰好只有 login / logout 以那两个词结尾,所以它一直是对的
// —— 但那是【巧合正确】:判定按子串建模,而新增 action 是别人在别的文件里做的事。
//
// 这个 handler 挂着的动作里有:
//   getdb          导出整个 settings 表 → 含 JWT 签名密钥 secret 与 Cloudflare API Token
//   importdb       用上传的库覆盖本机
//   cfCredentials  读 Cloudflare 凭据
// 一个叫 relogin / autologin / sublogin 的新 action 会静默地完全不鉴权,
// 而加它的人不会想到来看这个中间件,中间件也不会因此变红。
//
// setting.go 的注释写明了安全模型:「token 存 base64 仅做一层混淆 ——
// 真正的安全是 DB 文件 owner-only 权限 + 面板登录鉴权」。
// 文件权限那条腿是成立的(db.go 的 MkdirAll 0700 + install.sh chmod 700)。
// 这里守的是【另一条腿】。

import "testing"

func TestAuthExemptIsExactMatchOnly(t *testing.T) {
	// 豁免的恰好是这两个,一个不多
	for _, a := range []string{"login", "logout"} {
		if !isAuthExemptAction("", a) || !isAuthExemptAction(a, "") {
			t.Errorf("%q 应当豁免登录(它就是登录/登出本身)", a)
		}
	}

	// 🩸 这一组是本守卫的全部意义:它们【包含】login/logout,
	// 后缀匹配会把它们全部放行,精确匹配一个都不放。
	lookalikes := []string{
		"relogin", "autologin", "sublogin", "xlogin", "adminlogin",
		"relogout", "forcelogout", "sublogout",
		"login2", "loginx", // 前缀型:后缀匹配挡得住,但一起钉上
	}
	for _, a := range lookalikes {
		if isAuthExemptAction("", a) {
			t.Errorf("GET action %q 被判为免鉴权 —— 后缀/子串匹配的典型失效。\n"+
				"本 handler 挂着 getdb(导出含 JWT 密钥与 CF Token 的整库)、importdb、cfCredentials。", a)
		}
		if isAuthExemptAction(a, "") {
			t.Errorf("POST action %q 被判为免鉴权 —— 同上", a)
		}
	}

	// 敏感动作必须逐个确认不在豁免名单里(不是靠"我记得没加")
	for _, a := range []string{"getdb", "importdb", "cfCredentials", "cfSetCredentials",
		"tokens", "addToken", "settings", "save", "restartApp", "users", "changePass"} {
		if isAuthExemptAction("", a) || isAuthExemptAction(a, "") {
			t.Errorf("敏感 action %q 居然免鉴权", a)
		}
	}

	// 空 action(路由没匹配到参数)不得豁免 —— fail-closed
	if isAuthExemptAction("", "") {
		t.Error("空 action 被判免鉴权 —— 兜底方向反了,应当 fail-closed")
	}
}

// POST 与 GET 两个参数只会有一个非空(路由是 /:postAction 与 /:getAction)。
// 钉住:POST 非空时以它为准,不会因为 getAction 恰好是 "login" 而误放行。
func TestAuthExemptPrefersPostActionWhenPresent(t *testing.T) {
	if isAuthExemptAction("getdb", "login") {
		t.Error("POST getdb 因为 getAction 是 login 而被放行 —— 两个参数的优先级搞错了")
	}
}
