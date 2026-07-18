# NexCore s-ui

[简体中文](README.md) | [English](README.en.md) | **日本語** | [繁體中文](README.zh-TW.md)

[alireza0/s-ui](https://github.com/alireza0/s-ui) をベースに二次開発した sing-box ノード管理パネル。
**API ファースト + 自動デプロイ + マスターパネル連携**を目標としている。自前のノードサーバーにデプロイし、業務システムやプロキシスケジューリングシステムに被管理ノードとして組み込む用途に適しており、個人のスタンドアロン運用にも向いている。

> オリジナル版との違い:フロントエンドを Vue 3 + Element Plus で完全リライト、無人インストールの install.sh / update.sh、
> デフォルト認証情報は全てランダム生成、初回インストール時からランダムポート、`/api/v1/*` の完全な REST 体系かつ
> **[nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) マスターパネル向け連携コードと 100% 互換**、
> Cloudflare API Token による DNS-01 ワンクリック証明書発行 + 自動更新、API 呼び出しログ監査、
> API ドキュメント内蔵。パス / サービス名 / ポートは上流の `s-ui` から完全に独立しており、同一マシンでの共存が可能。

---

## システム要件

- **推奨**:**Ubuntu 24.04 LTS**(本リポジトリの主要テスト対象。install → upgrade → 全プロトコルのインバウンド / アウトバウンド / 中継までエンドツーエンドで検証済み)
- **互換**:Ubuntu 20.04+ / Debian 11+ / CentOS Stream 8+ / OpenCloudOS / systemd を備えた任意のモダン Linux
- **アーキテクチャ**:`amd64` / `arm64` / `386` / `armv5` / `armv6` / `armv7` / `s390x`
- **バイナリ**:GitHub Actions CI で **musl 静的リンクビルド**(Bootlin toolchain)を使用しており、**ホストの glibc バージョンに依存しない**。Ubuntu 20.04 のような古いディストリビューションでも release パッケージがそのまま動作する
- **実行ユーザー**:root(80/443 の特権ポートへの bind + ACME 証明書の書き込み + tun デバイスの作成に必要)

---

## ワンライナーインストール

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
```

バージョン指定:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
```

強制再インストール(バイナリを上書き、**db は保持**):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
```

インストール完了後、スクリプトが**ログイン情報を直接表示する**(ランダムユーザー名 / ランダムパスワード / パネル URI)。表示例:

```
═════════════════════════════════════════════
  nexcore-s-ui 已部署
═════════════════════════════════════════════
Current panel settings:
        Panel port:      3095
        Panel path:      /app/

访问地址:
http://1.2.3.4:3095/app/

首装明文凭据 (★ 立即记录,后续只能 nexcore-s-ui 菜单重置):
  用户名: admin_a3f9k2
  密 码:  9KqL4mPzN2vR
```

認証情報の平文表示は**この一度きり**であり、ターミナルを閉じると二度と確認できない — 直ちに記録すること。

対応 CPU アーキテクチャ:`amd64` / `386` / `arm64` / `armv7` / `armv6` / `armv5` / `s390x`(Linux + systemd)。

---

## 管理コマンド(`nexcore-s-ui` CLI)

### サービス制御
```text
nexcore-s-ui                  进入交互菜单
nexcore-s-ui start|stop|restart  启停服务
nexcore-s-ui status           状态摘要
nexcore-s-ui enable|disable   开机自启
nexcore-s-ui log              查看日志
```

### インストールライフサイクル
```text
nexcore-s-ui install [tag]    安装 / 升级
nexcore-s-ui update [tag]     等价于 install (db 自动保留)
nexcore-s-ui uninstall        卸载(连数据一起删)
```

### 認証情報 / ポート

```text
sui admin -show                              显示当前管理员
sui admin -username <u> -password <p>        修改账号密码
sui setting -port <N>                        改面板端口
sui setting -path </app/>                    改面板路径
sui setting -subPort <N>                     改订阅端口
sui setting -show                            显示所有 settings
sui uri                                      打印面板访问 URL(含 LAN / 公网)
```

> `sui` は下層のバイナリ(`/usr/local/nexcore-s-ui/sui`)、`nexcore-s-ui` は systemd /
> インストール層のラッパーである。両者は併存する。

完全なメニュー:`nexcore-s-ui help`。

---

## パネル機能

- **稼働状況ダッシュボード** — CPU / メモリ / ディスク / ネットワーク速度 / sing-box 稼働状態 / Goroutine 数
- **インバウンド管理** — 対応プロトコル:VLESS / VMess / Trojan / Shadowsocks / ShadowTLS /
  Hysteria(2)/ Naive / TUIC / AnyTLS / WireGuard / Tailscale / Warp / Tor /
  SSH / Reality / ECH / 全 XTLS
- **ルーティング / ブロックルール** — ルールセット + ルールの二層管理、**ワンクリックテンプレート**:広告ブロック / マルウェア /
  フィッシング / 中国本土直結 / プライベート IP 直結 / 推奨セット
- **クライアントサブスクリプション** — ネイティブリンク、JSON、Clash + メタ情報(トラフィック / 上り下り / 有効期限)、
  QR コード内蔵
- **TLS センター** — 通常証明書 + Reality + ECH + ACME(**Cloudflare ワンクリック自動発行**を含む)
- **トラフィック統計** — インバウンド / アウトバウンド / ユーザーの三次元集計 + クライアントトラフィック Top 5 ランキング
- **API コンソール**(本リポジトリで新規追加)
  - **Token 管理**:名前付き token、TTL、失効、平文の一度きり表示
  - **呼び出しログ**:`/apiv2/*` および `/api/v1/*` への各呼び出しの method/path/status/
    latency/IP/Token 備考をすべて DB に記録。フィルタ + ページネーション + ワンクリック全消去に対応
  - **API ドキュメント**:フロントエンドに組み込み。ベース URL + curl サンプル + 全エンドポイント早見表 +
    `/api/v1` 互換マッピング

---

## Cloudflare ワンクリック TLS 発行

パネル → **TLS 設定** → **Cloudflare ワンクリック発行**で、3 ステップでドメイン解決 + 証明書発行が完了する:

1. **Token + メールアドレス** — Cloudflare API Token(権限 `Zone:DNS:Edit + Zone:Read`、
   Global Key も対応)を貼り付け、ACME 登録用メールアドレスを入力
2. **DNS** — ルートドメインを選択、プレフィックス方式を選択(ランダム / カスタム / ルートドメイン)、パブリック IP を入力(ワンクリック
   自動取得可)、CF リバースプロキシ経由にするかを決定
3. **発行** — TLS 設定に名前を付けて送信。完了

裏側の動作:パネルが CF API を呼び出して A レコードを追加 → sing-box の ACME-via-Cloudflare TLS 設定を書き込み →
sing-box 起動時に内蔵 ACME クライアントが DNS-01 チャレンジで証明書を発行、**以降は自動更新**。
Token は永続化されない(sing-box への組み込み用途にのみ受け渡す)。

API での実行版:

```bash
TOKEN=...  ; CF=...  ; BASE=http://node:3095/app/api/v1

# 1) 查可签发的 zone
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST -d "{\"token\":\"$CF\"}" $BASE/sui/cloudflare/zones

# 2) 加 A 记录(随机前缀)
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/dns/upsert-a -d "{
    \"token\":\"$CF\",\"zoneId\":\"<zone-id>\",
    \"random\":true,\"prefix\":\"nodeA\",
    \"ip\":\"1.2.3.4\",\"proxied\":false}"

# 3) 生成内嵌 ACME 配置的 TLS 记录
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/tls/issue -d "{
    \"name\":\"cf-auto\",\"fqdn\":\"nodeA-x9k3m2.example.com\",
    \"email\":\"a@b.c\",\"token\":\"$CF\"}"
```

---

## REST API

三層が併存し、それぞれ用途が異なる:

| プレフィックス | 認証 | レスポンス形式 | 用途 |
|---|---|---|---|
| `/api/*` | session cookie | `{success, msg, obj}` | パネル UI 自身 |
| `/apiv2/*` | Bearer / X-API-Token / Token | `{success, msg, obj}` | 汎用スクリプト連携 |
| `/api/v1/*` | Bearer / X-API-Token | `{data}` / `{error,code,message,details}` | **`nexcore-x-ui` 互換**、マスターパネルから直接接続可能 |

完全なドキュメントはパネルに組み込まれており、ログイン後 **API 管理 → API ドキュメント**から参照できる。エンドポイント一覧(`/api/v1`):

| リソース | エンドポイント |
|---|---|
| Liveness | `GET /api/v1/health` |
| 認証セルフチェック | `GET /api/v1/me` |
| Server | `GET /server/status` |
| sing-box | `GET /xray/status` · `POST /xray/restart` · `GET /xray/config` · `GET /xray/logs`(`xray` という命名はマスターパネル互換のため) |
| Inbounds | `GET\|POST /inbounds` · `GET\|PUT\|DELETE /inbounds/:id` |
| Outbounds | `GET\|POST /outbounds` · `GET\|PUT\|DELETE /outbounds/:id` |
| Endpoints / Services / TLS | `GET /endpoints` · `GET /services` · `GET /tls` |
| Clients | `GET /clients` · `GET /clients/:identifier/traffic` · `POST /clients/:identifier/reset-traffic` |
| Onlines | `GET /onlines` · `GET /online-ips[/:tag]` · `GET /online-ips-by-email` |
| Traffic | `GET /traffic` · `GET /traffic/live` |
| Tokens | `GET\|POST /tokens` · `DELETE /tokens/:id` |
| Settings | `GET\|PATCH /settings` |
| Access logs | `GET\|DELETE /access-logs` |
| System | `POST /system/restart-panel` |
| Cloudflare(s-ui only) | `POST /sui/cloudflare/zones` · `POST /sui/cloudflare/dns/upsert-a` · `POST /sui/cloudflare/tls/issue` |
| sing-box raw(s-ui only) | `GET /sui/singbox/raw-config` · `GET /sui/subscription-uri` |

業務システムからの接続例:

```bash
TOKEN=...  ; BASE=http://node:3095/app/api/v1

# 健康
curl $BASE/health

# 当前身份
curl -H "Authorization: Bearer $TOKEN" $BASE/me

# 列出所有入站
curl -H "Authorization: Bearer $TOKEN" $BASE/inbounds

# 改面板订阅端口
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X PATCH $BASE/settings -d '{"subPort":3199}'

# 客户端流量
curl -H "Authorization: Bearer $TOKEN" $BASE/clients/alice/traffic
```

---

## nexcore-x-ui マスターパネルとの連携

`/api/v1` は [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) の REST 形態を完全にミラーしている:
**同一のパスレイアウト、同一の認証ヘッダー、同一のレスポンス形式、同一のステータスコード、同一のエラーコード命名、unix ミリ秒タイムスタンプ**。
x-ui 向けに書いたマスターパネル連携コードは、そのまま本ノードに向けるだけで修正不要:

```diff
- HOST=https://x-node.example.com/api/v1     # nexcore-x-ui 节点
+ HOST=https://s-node.example.com/app/api/v1 # nexcore-s-ui 节点
  # 同样的 Authorization: Bearer <token>
  # 同样的 {data} / {error,code,message} 响应壳
  # 同样的 HTTP 状态码语义
```

差異は schema 層のみ:`/inbounds` が返す settings は sing-box プロトコル(xray ではない)であり、
マスターパネル側でのレンダリングは `/health` が返す `impl` フィールド(`nexcore-s-ui` vs `nexcore-x-ui`)で
分岐すればよい。

---

## オンライン更新

CLI(インストール済みマシン):

```bash
nexcore-s-ui update            # 升级到最新 release
nexcore-s-ui update v1.0.0     # 升级 / 降级到指定 tag
```

ワンライナースクリプト(CLI の事前インストール不要。自動化された一括アップグレードに最適):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
```

`update.sh` と `install.sh` の違い:`db/` には触れない(データベース + TLS + クライアント記録を完全保持)、
システム依存関係を再インストールしない、systemd unit は release 側で変更があった場合のみ旧版をバックアップしたうえで更新する。実行内容は:
**tarball のダウンロード → SHA256 検証 → サービス停止 → sui + bin/ + service の置き換え →
migrate → サービス起動**のみ。完全な再インストールには `install.sh --force` を使うこと。

---

## 設定ファイルの場所

| パス | 内容 |
|---|---|
| `/usr/local/nexcore-s-ui/sui` | メインバイナリ |
| `/usr/local/nexcore-s-ui/bin/sing-box` | sing-box 子プロセス |
| `/usr/local/nexcore-s-ui/db/nexcore-s-ui.db` | sqlite データベース(サブスクリプション / クライアント / インバウンド / TLS / Token / 呼び出しログ) |
| `/etc/systemd/system/nexcore-s-ui.service` | systemd ユニット |
| `/usr/bin/nexcore-s-ui` | 管理 CLI(`/usr/local/nexcore-s-ui/nexcore-s-ui.sh` を指す) |

環境変数:

| 変数 | デフォルト | 説明 |
|---|---|---|
| `SUI_DB_FOLDER` | `<binary 目录>/db` | データベースフォルダのパス |
| `SUI_BIN_FOLDER` | `bin` | sing-box 子プロセスのディレクトリ |
| `SUI_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUI_DEBUG` | `false` | デバッグモード |
| `GH_OWNER` / `GH_REPO` | `DoBestone` / `nexcore-s-ui` | 自己更新の取得元(install.sh / update.sh) |
| `INSTALL_DIR` | `/usr/local/nexcore-s-ui` | カスタムインストールディレクトリ(install.sh / update.sh) |

---

## 上流 alireza0/s-ui との共存

`nexcore-s-ui` はパス / サービス名 / データベース / ポート / コマンド名 / ブラウザ cookie のすべてで上流の
`s-ui` から**完全に独立**しており、同一マシンに同時インストールしても互いに干渉しない。

| 項目 | 上流 `s-ui` | `nexcore-s-ui` |
|---|---|---|
| インストールディレクトリ | `/usr/local/s-ui/` | `/usr/local/nexcore-s-ui/` |
| データベース | `db/s-ui.db` | `db/nexcore-s-ui.db` |
| systemd | `s-ui.service` | `nexcore-s-ui.service` |
| 管理コマンド | `/usr/bin/s-ui` | `/usr/bin/nexcore-s-ui` |
| デフォルトパネルポート | 2095 | **3095** |
| デフォルトサブスクリプションポート | 2096 | **3096** |
| ブラウザ cookie | `s-ui` | `nexcore-s-ui` |
| `sui -v` | `S-UI Panel 1.4.x` | `nexcore-s-ui 1.0.0` |

2 つのパネルはそれぞれ別のポートを使用する必要がある(デフォルト値はあらかじめずらしてある)。`nexcore-s-ui` をアンインストールしても
`/usr/local/s-ui/` には一切触れない。

---

## 開発

```bash
git clone https://github.com/DoBestone/nexcore-s-ui.git
cd nexcore-s-ui
./build.sh
```

`build.sh` は順に:`cd frontend && npm i && npm run build` → `cp -R frontend/dist/* web/html/` →
`go build -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale" -o sui main.go` を実行する。
最終成果物は `./sui`(Linux / macOS arm64 で約 75 MB)。

フロントエンドのみの開発:

```bash
cd frontend
npm i
npm run dev    # vite dev server, 默认 :3000,代理 /app/api → :3095
```

tag を打つと CI がトリガーされる(GitHub Actions が 7 つの linux arch + 2 つの windows arch をクロスコンパイルし、
release を自動公開):

```bash
git tag v1.0.1 && git push origin v1.0.1
```

---

## 謝辞

Forked from [alireza0/s-ui](https://github.com/alireza0/s-ui)。
sing-box は [SagerNet](https://github.com/SagerNet/sing-box) による。
フロントエンドコンポーネントは [Element Plus](https://element-plus.org/) + [Vue 3](https://vuejs.org/) をベースにしている。
DNS / ACME の自動化には [Cloudflare API](https://developers.cloudflare.com/api/) を使用。

GPL v3。
