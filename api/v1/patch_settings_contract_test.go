package v1

// PATCH /api/v1/settings 的字段契约。
//
// 🩸 2026-08-29 事故:该端点的 body 结构体只有 Port / Path 两个字段,而主控的
// SyncNodeNameToNode 发的是 {"nodeName": "..."}。ShouldBindJSON 会把结构体里
// **没有的字段静默丢弃**,然后照样 OK(c, {"updated": true})。
//
// 于是:主控收到 200 → 日志打印「节点名已同步到节点 panel」→ 运营以为配好了,
// 而节点上 settings.nodeName 一直是空。空 nodeName 会让分享链接的 remark
// 回退成 inbound.Tag,用户在客户端里看到的是 "nx-vless-reality-GsKvQKoi-16-1"
// 这种内部标识,而不是「美国节点Two(住宅IP)」。
//
// **谎报成功比不报更糟** —— 不报至少会有人去查。
//
// 本守卫钉两件事:
//  1. body 结构体里声明的每个字段都必须在 handler 里被真正消费(声明了不接线 = 同一个坑);
//  2. nodeName 必须在契约里(它是主控唯一会 PATCH 的业务字段)。

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// patchSettingsBody 取 patchSettings 里那个内联 body 结构体的字段名 + json tag,
// 以及函数体源码(不带注释重打,避免注释里提到字段名把守卫骗过去)。
func patchSettingsBody(t *testing.T) (fields map[string]string, body string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "v1.go", nil, 0) // 0 = 不解析注释
	if err != nil {
		t.Fatalf("解析 v1.go 失败: %v", err)
	}
	fields = map[string]string{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "patchSettings" || fd.Body == nil {
			continue
		}
		var sb strings.Builder
		if err := printer.Fprint(&sb, fset, fd.Body); err != nil {
			t.Fatalf("打印函数体失败: %v", err)
		}
		body = sb.String()
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fl := range st.Fields.List {
				tag := ""
				if fl.Tag != nil {
					raw := strings.Trim(fl.Tag.Value, "`")
					if i := strings.Index(raw, `json:"`); i >= 0 {
						rest := raw[i+6:]
						if j := strings.Index(rest, `"`); j >= 0 {
							tag = strings.Split(rest[:j], ",")[0]
						}
					}
				}
				for _, nm := range fl.Names {
					fields[nm.Name] = tag
				}
			}
			return true
		})
		break
	}
	// 自证:函数或结构体找不到就必须 Fatal —— t.Skip 会让守卫在自己失效那刻静默。
	if body == "" {
		t.Fatal("在 v1.go 里找不到 patchSettings —— 它可能被改名或挪走了,守卫已失效")
	}
	if len(fields) == 0 {
		t.Fatal("patchSettings 里没解析到任何 body 字段 —— 结构体可能被提到函数外,守卫已失效")
	}
	return fields, body
}

func TestPatchSettingsDeclaredFieldsAreAllConsumed(t *testing.T) {
	fields, body := patchSettingsBody(t)
	t.Logf("契约字段: %v", fields)

	for name, tag := range fields {
		// 每个声明的字段都必须在函数体里被读到。只声明不消费 =
		// 调用方以为设置成功、实际被丢弃,而 HTTP 依然 200。
		if !strings.Contains(body, "b."+name) {
			t.Fatalf("body 字段 %s(json:%q)声明了却没在 handler 里消费 —— "+
				"ShouldBindJSON 会收下它然后丢掉,调用方拿到 200 却什么都没生效", name, tag)
		}
	}
}

func TestPatchSettingsAcceptsNodeName(t *testing.T) {
	fields, body := patchSettingsBody(t)

	var found string
	for name, tag := range fields {
		if tag == "nodeName" {
			found = name
		}
	}
	if found == "" {
		t.Fatal("PATCH /settings 的 body 里没有 json:\"nodeName\" —— " +
			"主控 SyncNodeNameToNode 发的就是这个键,缺了它会被静默丢弃并回 200," +
			"表现为『主控说同步成功、节点上却一直是空』,而用户看到的线路名会退化成 inbound.Tag")
	}
	if !strings.Contains(body, "SetNodeName(") {
		t.Fatalf("字段 %s 存在但 handler 没调 SetNodeName —— 收下了不落库,等于没接", found)
	}
	// nodeName 允许显式置空(空 = 回到 tag 兜底,是合法状态),
	// 所以不能像 Path 那样加 `!= ""` 的判空,否则「清空节点名」这个操作会静默失效。
	if strings.Contains(body, "*b."+found+" != \"\"") {
		t.Fatalf("handler 对 %s 加了非空判断 —— 那会让『把节点名清空』这个合法操作静默失效", found)
	}
}
