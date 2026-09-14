package service

import (
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// 新增的设置 key 在存量库里没有对应行,而保存路径是
// `UPDATE settings SET value=? WHERE key=?` —— 行不存在时 RowsAffected=0,
// 面板上改完点保存会返回成功而值纹丝不动。
//
// 兜底靠 GetAllSetting 里那段"缺 key 就按 defaultValueMap 补一行"。
// 本用例钉住这条链:新 key 必须在读取设置后真的落到表里,否则它就只是个
// 永远改不动的摆设。
func TestRetentionKeysPersistOnRead(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "keys.db")); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	d := database.GetDB()
	keys := []string{"apiLogAge", "changesAge"}

	// 模拟存量库:这两行本来不存在
	for _, k := range keys {
		if err := d.Exec("DELETE FROM settings WHERE key = ?", k).Error; err != nil {
			t.Fatalf("清 %s 失败: %v", k, err)
		}
	}
	// 夹具自证:确认真的清掉了,否则下面的断言毫无信息量
	for _, k := range keys {
		var n int64
		if err := d.Model(&model.Setting{}).Where("key = ?", k).Count(&n).Error; err != nil {
			t.Fatalf("数 %s 失败: %v", k, err)
		}
		if n != 0 {
			t.Fatalf("夹具不成立:%s 清理后仍有 %d 行", k, n)
		}
	}

	var ss SettingService
	if _, err := ss.GetAllSetting(); err != nil {
		t.Fatalf("GetAllSetting 失败: %v", err)
	}

	for _, k := range keys {
		var n int64
		if err := d.Model(&model.Setting{}).Where("key = ?", k).Count(&n).Error; err != nil {
			t.Fatalf("数 %s 失败: %v", k, err)
		}
		if n != 1 {
			t.Fatalf("读取设置后 %s 应已落库(1 行),实得 %d —— "+
				"这个 key 在面板上会改不动:保存走 UPDATE,没有行就静默无效", k, n)
		}
	}
}

// 保留天数的默认值是这次轻量化的核心结论,用例把它钉死,防止后来者
// "顺手调回 30" 而没人发现磁盘又开始涨。
func TestRetentionDefaults(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "defaults.db")); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	var ss SettingService

	for _, tc := range []struct {
		name string
		get  func() (int, error)
		want int
	}{
		{"trafficAge", ss.GetTrafficAge, 7},
		{"apiLogAge", ss.GetApiLogAge, 7},
		{"changesAge", ss.GetChangesAge, 30},
	} {
		got, err := tc.get()
		if err != nil {
			t.Fatalf("%s 读取失败: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s 默认值应为 %d 天,实得 %d", tc.name, tc.want, got)
		}
	}
}
