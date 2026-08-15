package service

import "testing"

// 回归护栏:AUDIT 里 /apiv2 认证绕过的根因是"掩码值 **** 被当成 token 做等值比较"。
// MatchToken 必须对掩码值、空值一律拒绝,同时保持 hash / legacy 明文双模式可用。
func TestMatchToken(t *testing.T) {
	plain := "abcDEF0123456789abcDEF0123456789"
	hashed := hashToken(plain)

	cases := []struct {
		name   string
		stored string
		raw    string
		want   bool
	}{
		{"hashed 存储 + 正确明文", hashed, plain, true},
		{"hashed 存储 + 错误明文", hashed, "wrong-token-value", false},
		{"legacy 明文存储 + 正确明文", plain, plain, true},
		{"legacy 明文存储 + 错误明文", plain, "wrong-token-value", false},
		// 认证绕过回归:掩码值既不能作为 stored 被命中,也不能作为 raw 命中任何记录
		{"掩码值当 raw 打 hashed 记录", hashed, "****", false},
		{"掩码值当 raw 打明文记录", plain, "****", false},
		{"掩码值同时出现在两侧", "****", "****", false},
		{"空 raw", hashed, "", false},
		{"空 stored", "", plain, false},
		{"两侧全空", "", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchToken(c.stored, c.raw); got != c.want {
				t.Fatalf("MatchToken(%q, %q) = %v, want %v", c.stored, c.raw, got, c.want)
			}
		})
	}
}
