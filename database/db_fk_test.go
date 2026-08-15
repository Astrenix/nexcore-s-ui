package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database/model"
)

// 验证第二轮重新启用的 SQLite 外键在完整 gorm 链路下的行为:
//   - 无 TLS 入站(TlsId=nil → NULL)必须能保存(第一轮回退的直接原因)
//   - 绑定真实 TLS 的入站能保存
//   - 指向不存在 tls 的孤儿引用被外键拒绝
func TestForeignKeyInboundTls(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fk_test.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	d := GetDB()

	tls := model.Tls{Name: "real", Server: json.RawMessage("{}"), Client: json.RawMessage("{}")}
	if err := d.Create(&tls).Error; err != nil {
		t.Fatalf("建 TLS 失败: %v", err)
	}

	// 无 TLS 入站:TlsId=nil → 列 NULL,外键放行
	noTls := model.Inbound{Tag: "no-tls", Type: "vless", TlsId: nil, Options: json.RawMessage("{}")}
	if err := d.Create(&noTls).Error; err != nil {
		t.Fatalf("无 TLS 入站应保存成功,却被拒: %v", err)
	}

	// 绑定真实 TLS 的入站
	withTls := model.Inbound{Tag: "with-tls", Type: "vless", TlsId: &tls.Id, Options: json.RawMessage("{}")}
	if err := d.Create(&withTls).Error; err != nil {
		t.Fatalf("有 TLS 入站应保存成功,却被拒: %v", err)
	}

	// 孤儿引用:指向不存在的 tls.id → 外键必须拒绝
	orphanId := uint(99999)
	orphan := model.Inbound{Tag: "orphan", Type: "vless", TlsId: &orphanId, Options: json.RawMessage("{}")}
	if err := d.Create(&orphan).Error; err == nil {
		t.Fatal("孤儿 tls_id 应被外键拒绝,却保存成功了(外键未生效)")
	}
}
