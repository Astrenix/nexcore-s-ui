package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
}

// bcrypt cost 12:登录是低频操作,12 在现代 CPU 约 200-300ms,可接受;
// cost 10 是 OWASP 最低门槛,12 给点冗余应对未来 GPU 攻击。
const bcryptCost = 12

// hashPassword 用 bcrypt 生成 password hash。失败返回原密码 — 永远不应发生
// (bcrypt.GenerateFromPassword 仅在 cost 越界时报错),但 fallback 保留登录能力。
func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// isBcryptHash 用前缀 + 长度粗筛 — bcrypt hash 永远 60 字符,以 "$2a$"/"$2b$"/"$2y$" 起头。
// 用来区分 DB 里现有明文密码 vs 已升级到 hash 的密码,启动迁移的核心判别逻辑。
func isBcryptHash(s string) bool {
	if len(s) != 60 {
		return false
	}
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
}

// verifyPassword 比对密码。优先 bcrypt(常量时间),失败时若 stored 不是 bcrypt 就 fallback
// 比明文(老 DB 兼容,首次匹配后调用方应升级)。
func verifyPassword(stored, plain string) (matched bool, isLegacyPlain bool) {
	if isBcryptHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil, false
	}
	// stored 不像 bcrypt → 视为旧明文,直接比较;match 时返回 isLegacyPlain=true,
	// 让调用方负责升级到 bcrypt。
	return stored == plain, stored == plain
}

// hashToken sha256 hex 给 v1 token 落库前用 — 跟密码不一样,token 是高熵随机串
// (32 字节随机),用 sha256 单向就够,不需要 bcrypt 那种 work factor。
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// looksLikeTokenHash 64 字符 hex = sha256 hex 输出。区分明文 token vs hashed
// token 的依据 — 老 DB 里 common.Random(32) 生成的是 32-char 字母数字混合串,
// 长度跟 sha256 hex 不撞,且 common.Random 字符集含大写,sha256 hex 全小写,
// 双保险不会误判。
func looksLikeTokenHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// HashTokenForCompare 暴露给 api/v1 中间件:把客户端送来的 raw token sha256,
// 跟 DB 里的 hashed token 等值比。中间件保留对 legacy 明文 token 的双模式
// 比较(直接比 raw == stored),所以这里只做 hash 一种路径。
func HashTokenForCompare(raw string) string {
	return hashToken(raw)
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return common.NewError("username can not be empty")
	} else if password == "" {
		return common.NewError("password can not be empty")
	}
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashed
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashed
	return db.Save(user).Error
}

func (s *UserService) Login(username string, password string, remoteIP string) (string, error) {
	// AUDIT.md C4:per-IP 阶梯延迟,5 次起 5s,10 次起 15s,20 次起 60s。
	// CheckAndDelay 在 throttle 内 sleep,attacker 自动脚本会被熬到放弃;
	// 合法用户即便偶尔输错 4 次也无感(< 5 次无延迟)。
	throttle := LoginThrottle()
	throttle.CheckAndDelay(remoteIP)

	user := s.CheckUser(username, password, remoteIP)
	if user == nil {
		throttle.MarkFailure(remoteIP)
		fc := throttle.FailureCount(remoteIP)
		if fc >= 5 {
			logger.Warning("login throttle: ", fc, " consecutive failures from IP ", remoteIP)
		}
		return "", common.NewError("wrong user or password! IP: ", remoteIP)
	}
	throttle.MarkSuccess(remoteIP)
	return user.Username, nil
}

// CheckUser 用 bcrypt 验证密码;若 DB 里仍是旧明文,匹配后顺手升级到 bcrypt。
//
// 安全边界:不再用 `WHERE password=?` 这种明文 SQL 比对(原版做法),
// 改成按 username 查出后内存里 bcrypt.Compare,避免 SQL 比对路径泄漏明文密码。
func (s *UserService) CheckUser(username, password, remoteIP string) *model.User {
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).Where("username = ?", username).First(user).Error
	if database.IsNotFound(err) {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err, " IP: ", remoteIP)
		return nil
	}
	matched, legacyPlain := verifyPassword(user.Password, password)
	if !matched {
		return nil
	}

	// 旧明文匹配成功 → 顺手升级到 bcrypt(失败仅 warning,登录正常返回)
	if legacyPlain {
		if hashed, hErr := hashPassword(password); hErr == nil {
			if uErr := db.Model(model.User{}).Where("id = ?", user.Id).Update("password", hashed).Error; uErr == nil {
				user.Password = hashed
				logger.Info("upgraded plaintext password to bcrypt for user: ", username)
			} else {
				logger.Warning("upgrade password to bcrypt failed for ", username, ": ", uErr)
			}
		}
	}

	lastLoginTxt := time.Now().Format("2006-01-02 15:04:05") + " " + remoteIP
	if err := db.Model(model.User{}).Where("username = ?", username).Update("last_logins", &lastLoginTxt).Error; err != nil {
		logger.Warning("unable to log login data", err)
	}
	return user
}

func (s *UserService) GetUsers() (*[]model.User, error) {
	var users []model.User
	db := database.GetDB()
	err := db.Model(model.User{}).Select("id,username,last_logins").Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return &users, nil
}

// ChangePass 必须先校验旧密码再写新 hash;旧密码走 verifyPassword 跟 CheckUser 一致
// (bcrypt + 旧明文 fallback),新密码强制 bcrypt。
func (s *UserService) ChangePass(id string, oldPass string, newUser string, newPass string) error {
	if newPass == "" {
		return common.NewError("password can not be empty")
	}
	db := database.GetDB()
	user := &model.User{}
	if err := db.Model(model.User{}).Where("id = ?", id).First(user).Error; err != nil {
		return err
	}
	matched, _ := verifyPassword(user.Password, oldPass)
	if !matched {
		return common.NewError("old password mismatch")
	}
	hashed, err := hashPassword(newPass)
	if err != nil {
		return err
	}
	user.Username = newUser
	user.Password = hashed
	return db.Save(user).Error
}

func (s *UserService) LoadTokens() ([]byte, error) {
	db := database.GetDB()
	var tokens []model.Tokens
	err := db.Model(model.Tokens{}).Preload("User").Where("expiry == 0 or expiry > ?", time.Now().Unix()).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, t := range tokens {
		username := ""
		if t.User != nil {
			username = t.User.Username
		}
		// 不暴露 hashed token 也别误暴露明文 — 走 ****;运维需要 token 自己重置
		result = append(result, map[string]interface{}{
			"token":    "****",
			"expiry":   t.Expiry,
			"username": username,
			"desc":     t.Desc,
		})
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return jsonResult, nil
}

// LoadTokensForAuth 给 /apiv2 鉴权用 —— 返回 DB 里的**真实** token 列(hash 或
// legacy 明文),不掩码。跟 LoadTokens 分家是因为后者的 "****" 掩码是给前端展示
// 的,一旦被拿去做等值比较,任何人发 Token: **** 都能命中 → 认证绕过。
// 结果只进内存鉴权表,绝不出网。
func (s *UserService) LoadTokensForAuth() ([]model.Tokens, error) {
	db := database.GetDB()
	var tokens []model.Tokens
	err := db.Model(model.Tokens{}).Preload("User").
		Where("expiry == 0 or expiry > ?", time.Now().Unix()).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

// MaskedToken 是 token 对外展示时的占位符。定义在这里是为了让"掩码值永远不是
// 有效凭证"成为一条可被引用、可被测试的不变式,而不是散落在各处的字面量。
const MaskedToken = "****"

// MatchToken 双模式 constant-time 比对,语义跟 api/v1 中间件的 resolveToken 一致:
// DB 存的是 sha256 hex(新 token)就跟 sha256(raw) 比,否则按 legacy 明文比。
func MatchToken(stored string, raw string) bool {
	if stored == "" || raw == "" {
		return false
	}
	// 掩码值任何一侧出现都直接拒绝:正常写入路径不会把 "****" 落库,但一旦
	// 有人(或将来某个 bug)把展示值喂进鉴权表,它绝不能成为通行证。
	if stored == MaskedToken || raw == MaskedToken {
		return false
	}
	compare := raw
	if looksLikeTokenHash(stored) {
		compare = hashToken(raw)
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(compare)) == 1
}

func (s *UserService) GetUserTokens(username string) (*[]model.Tokens, error) {
	db := database.GetDB()
	var token []model.Tokens
	err := db.Model(model.Tokens{}).Select("id,desc,'****' as token,expiry,user_id").Where("user_id = (select id from users where username = ?)", username).Find(&token).Error
	if err != nil && !database.IsNotFound(err) {
		println(err.Error())
		return nil, err
	}
	return &token, nil
}

// AddToken 生成 32 字节随机明文 token,DB 只存 sha256 hash,**明文仅本次返回一次**。
// 客户端必须当场记录,丢失只能 ResetToken 重发。
func (s *UserService) AddToken(username string, expiry int64, desc string) (string, error) {
	db := database.GetDB()
	var userId uint
	err := db.Model(model.User{}).Where("username = ?", username).Select("id").Scan(&userId).Error
	if err != nil {
		return "", err
	}
	if expiry > 0 {
		expiry = expiry*86400 + time.Now().Unix()
	}
	plain := common.Random(32)
	token := &model.Tokens{
		Token:  hashToken(plain),
		Desc:   desc,
		Expiry: expiry,
		UserId: userId,
	}
	if err := db.Create(token).Error; err != nil {
		return "", err
	}
	return plain, nil
}

func (s *UserService) DeleteToken(id string) error {
	db := database.GetDB()
	return db.Model(model.Tokens{}).Where("id = ?", id).Delete(&model.Tokens{}).Error
}

// ResetToken 同 AddToken — DB 存 hash,返回明文一次。
func (s *UserService) ResetToken(id string) (string, error) {
	db := database.GetDB()
	t := &model.Tokens{}
	if err := db.Model(model.Tokens{}).Where("id = ?", id).First(t).Error; err != nil {
		return "", err
	}
	plain := common.Random(32)
	t.Token = hashToken(plain)
	if err := db.Save(t).Error; err != nil {
		return "", err
	}
	return plain, nil
}

// UpgradePlaintextPasswords 启动时跑一次 — 把所有 users.password 不是 bcrypt 形态的
// 升级到 bcrypt。失败的单条只打 Warning,不阻塞启动(避免某条 DB 行损坏让整个 panel 起不来)。
//
// 旧 token(明文)不强制升级 —— 用户脚本里硬编码了原明文,升级会让所有外部对接秒失效;
// 中间件保留双模式比较(legacy 明文 vs 新 sha256 hash),自然过渡:用户重置 / 新增的
// token 会自动落 hash。完整 audit story 见 AUDIT.md C1。
func UpgradePlaintextPasswords() {
	db := database.GetDB()
	if db == nil {
		return
	}
	var users []model.User
	if err := db.Model(model.User{}).Find(&users).Error; err != nil {
		logger.Warning("UpgradePlaintextPasswords: load users:", err)
		return
	}
	upgraded := 0
	for _, u := range users {
		if isBcryptHash(u.Password) {
			continue
		}
		hashed, err := hashPassword(u.Password)
		if err != nil {
			logger.Warning("UpgradePlaintextPasswords: hash for user ", u.Username, ":", err)
			continue
		}
		if err := db.Model(model.User{}).Where("id = ?", u.Id).Update("password", hashed).Error; err != nil {
			logger.Warning("UpgradePlaintextPasswords: write for user ", u.Username, ":", err)
			continue
		}
		upgraded++
	}
	if upgraded > 0 {
		logger.Info("UpgradePlaintextPasswords: upgraded ", upgraded, " plaintext password(s) to bcrypt")
	}
}
