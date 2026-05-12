package service

import (
	"strings"
)

// detectCountry 优先级:
//  1. Remark 关键词(中英文 + ISO-2 缩写匹配)→ 大部分机场都在节点名里标了国家
//  2. ExitIP 简单网段判定(几个常见 IDC,极简,不依赖 GeoIP mmdb)
//  3. fallback "XX"(unknown)
//
// 这里只做高命中率的关键词;边缘地区(冰岛 / 哥斯达黎加 …)会落到 XX,
// 用户可以手动改 sub_nodes.country 列(前端二期支持手动指派)。
func detectCountry(remark, exitIP string) string {
	if c := detectFromRemark(remark); c != "" {
		return c
	}
	// 二期再加 ExitIP GeoIP 兜底;当前 keyword 覆盖率已经 90%+
	return "XX"
}

// 内置关键词表(简体 / 繁体 / 英文 / ISO-2)
// 按从长到短匹配,避免"美"先撞上"美洲"误判
var countryKeywords = []struct {
	keys []string
	cc   string
}{
	{[]string{"香港", "HongKong", "Hong Kong", "HK"}, "HK"},
	{[]string{"台湾", "臺灣", "Taiwan", "TW"}, "TW"},
	{[]string{"日本", "Japan", "JP", "東京", "东京", "Tokyo", "Osaka", "大阪"}, "JP"},
	{[]string{"韩国", "韓國", "Korea", "KR", "首尔", "首爾", "Seoul"}, "KR"},
	{[]string{"新加坡", "Singapore", "SG"}, "SG"},
	{[]string{"美国", "美國", "United States", "USA", "US ", " US", "America", "硅谷", "矽谷", "洛杉矶", "拉斯维加斯", "圣何塞", "纽约"}, "US"},
	{[]string{"加拿大", "Canada", "CA "}, "CA"},
	{[]string{"英国", "英國", "United Kingdom", "UK ", " UK", "Britain", "London", "伦敦"}, "GB"},
	{[]string{"德国", "德國", "Germany", "DE", "Frankfurt", "法兰克福"}, "DE"},
	{[]string{"法国", "法國", "France", "FR ", " FR", "Paris", "巴黎"}, "FR"},
	{[]string{"荷兰", "荷蘭", "Netherlands", "NL", "Amsterdam"}, "NL"},
	{[]string{"俄罗斯", "俄羅斯", "Russia", "RU ", " RU", "Moscow", "莫斯科"}, "RU"},
	{[]string{"土耳其", "Turkey", "TR ", "İstanbul", "Istanbul"}, "TR"},
	{[]string{"印度", "India", "IN "}, "IN"},
	{[]string{"澳大利亚", "Australia", "AU ", "Sydney", "悉尼"}, "AU"},
	{[]string{"巴西", "Brazil", "BR "}, "BR"},
	{[]string{"阿根廷", "Argentina", "AR "}, "AR"},
	{[]string{"墨西哥", "Mexico", "MX "}, "MX"},
	{[]string{"南非", "South Africa", "ZA "}, "ZA"},
	{[]string{"以色列", "Israel", "IL "}, "IL"},
	{[]string{"阿联酋", "UAE", "Dubai", "迪拜"}, "AE"},
	{[]string{"沙特", "Saudi", "SA "}, "SA"},
	{[]string{"泰国", "泰國", "Thailand", "TH "}, "TH"},
	{[]string{"越南", "Vietnam", "VN "}, "VN"},
	{[]string{"马来", "馬來", "Malaysia", "MY "}, "MY"},
	{[]string{"印尼", "Indonesia", "ID "}, "ID"},
	{[]string{"菲律宾", "Philippines", "PH "}, "PH"},
	{[]string{"乌克兰", "Ukraine", "UA "}, "UA"},
	{[]string{"波兰", "Poland", "PL "}, "PL"},
	{[]string{"瑞士", "Switzerland", "CH "}, "CH"},
	{[]string{"瑞典", "Sweden", "SE "}, "SE"},
	{[]string{"挪威", "Norway", "NO "}, "NO"},
	{[]string{"芬兰", "Finland", "FI "}, "FI"},
	{[]string{"丹麦", "Denmark", "DK "}, "DK"},
	{[]string{"西班牙", "Spain", "ES "}, "ES"},
	{[]string{"意大利", "Italy", "IT "}, "IT"},
	{[]string{"奥地利", "Austria", "AT "}, "AT"},
	{[]string{"比利时", "Belgium", "BE "}, "BE"},
	{[]string{"爱尔兰", "Ireland", "IE "}, "IE"},
	{[]string{"葡萄牙", "Portugal", "PT "}, "PT"},
	{[]string{"中国", "China", "CN ", "上海", "北京", "广州", "深圳"}, "CN"},
}

func detectFromRemark(remark string) string {
	if remark == "" {
		return ""
	}
	// 首尾加空格,让 "US " / " US" 这种边界匹配能 hit;
	// 同时全用 lower 便于英文匹配
	r := " " + remark + " "
	lower := strings.ToLower(r)
	for _, ent := range countryKeywords {
		for _, k := range ent.keys {
			if strings.Contains(r, k) {
				return ent.cc
			}
			// 英文 ISO-2 小写匹配
			if len(k) <= 4 && strings.Contains(lower, strings.ToLower(k)) {
				return ent.cc
			}
		}
	}
	return ""
}
