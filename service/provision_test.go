package service

import "testing"

// TestSanitizeSubdomainPrefix 子域名前缀会被拼进真实 DNS 记录名与 Cloudflare
// API 的 query,脏输入的后果发生在别人的域名上。
//
// 这不是防攻击(输入来自主控管理员),是防手滑:一个 `.` 就能把节点域名建到
// 另一级子域下,一个 `&` 就能截断 "查同名记录" 的 query 让它查空、进而
// 每次交付都重复创建记录。
func TestSanitizeSubdomainPrefix(t *testing.T) {
	ok := []struct{ in, want string }{
		{"", ""},         // 空 → 交给 RandomSubdomain 回退成 "n"
		{"jp", "jp"},     //
		{"JP", "jp"},     // 大写归一化 —— DNS 标签本就大小写不敏感
		{"  jp  ", "jp"}, // 两侧空白剪掉
		{"jp-tokyo", "jp-tokyo"},
		{"node1", "node1"},
		{"a", "a"},
	}
	for _, c := range ok {
		got, err := sanitizeSubdomainPrefix(c.in)
		if err != nil {
			t.Errorf("sanitizeSubdomainPrefix(%q) 不该报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("sanitizeSubdomainPrefix(%q) = %q,期望 %q", c.in, got, c.want)
		}
	}

	bad := []struct{ in, why string }{
		{"jp.tokyo", "含点号 —— 会把记录建到另一级子域下"},
		{"jp&x", "含 & —— 截断 Cloudflare API 的 query"},
		{"jp#x", "含 # —— 同上"},
		{"jp x", "含空格 —— Cloudflare 会拒,且错误信息指不到原因"},
		{"jp/x", "含斜杠 —— 改变 API 路径"},
		{"jp?x", "含问号 —— 截断 query"},
		{"-jp", "以连字符开头,不是合法 DNS 标签"},
		{"jp-", "以连字符结尾,不是合法 DNS 标签"},
		{"这是中文", "非 ASCII"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "超过 24 字符(后面还要接 -8hex,总长要留在 63 字节标签上限内)"},
	}
	for _, c := range bad {
		if _, err := sanitizeSubdomainPrefix(c.in); err == nil {
			t.Errorf("sanitizeSubdomainPrefix(%q) 应当报错 —— %s", c.in, c.why)
		}
	}
}

// TestSanitizeSubdomainPrefixLengthBoundary 24 是允许的上界,25 不是。
//
// 单独测边界:这类"上限"最常见的写错方式是 > 与 >= 搞反,而它不会在
// 日常输入里暴露 —— 只有恰好卡在边界的那个值会。
func TestSanitizeSubdomainPrefixLengthBoundary(t *testing.T) {
	at := "abcdefghijklmnopqrstuvwx" // 24
	if len(at) != 24 {
		t.Fatalf("测试数据本身长度不对: %d", len(at))
	}
	if _, err := sanitizeSubdomainPrefix(at); err != nil {
		t.Errorf("24 字符应当被接受: %v", err)
	}
	if _, err := sanitizeSubdomainPrefix(at + "y"); err == nil {
		t.Error("25 字符应当被拒")
	}
}
