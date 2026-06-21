package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/sha3"
)

// RandString generate rand string with specified length
func RandString(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := []byte(str)
	var result []byte
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		result = append(result, data[r.Intn(len(data))])
	}
	return string(result)
}

// GenerateShortID 根据日期字符串生成6位唯一标识符
func GenerateShortID() string {

	// 使用时间戳作为哈希输入（更精确）
	timestamp := time.Now().UnixNano()

	// 使用SHA-1生成哈希（比MD5更安全）
	hash := sha1.Sum([]byte(fmt.Sprintf("%d", timestamp)))

	// 计算MD5哈希
	hashStr := hex.EncodeToString(hash[:])
	// 从哈希结果中提取6个字符（使用第8-13个字符）
	result := hashStr[7:13] // 注意索引从0开始
	return result
}

func RandomNumber(bit int) int {
	min := intPow(10, bit-1)
	max := intPow(10, bit) - 1

	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

func UintToString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)

}

// ReplacePathPrefix 替换路径前缀
func ReplacePathPrefix(fullPath string) string {
	basePath, _ := os.Getwd()
	if strings.HasPrefix(fullPath, basePath) {
		//return filepath.Join("media", strings.TrimPrefix(fullPath, basePath))
		return strings.TrimPrefix(fullPath, basePath)
	}
	return fullPath
}

func intPow(x, y int) int {
	result := 1
	for i := 0; i < y; i++ {
		result *= x
	}
	return result
}

func ContainsStr(slice []string, item string) bool {
	for _, e := range slice {
		if e == item {
			return true
		}
	}
	return false
}

// Stamp2str 时间戳转字符串
func Stamp2str(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return time.Unix(timestamp, 0).Format("2006-01-02 15:04:05")
}

// Str2stamp 字符串转时间戳
func Str2stamp(str string) int64 {
	if len(str) == 0 {
		return 0
	}

	layout := "2006-01-02 15:04:05"
	t, err := time.ParseInLocation(layout, str, time.Local)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func GenPassword(pass string, salt string) string {
	data := []byte(pass + salt)
	hash := sha3.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// 密码加密: pwdHash  同PHP函数 password_hash()
func PasswordHash(pwd string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(bytes), err
}

func IsWechatPrefix(s string) bool {
	if len(s) < 4 {
		return false
	}

	return strings.HasPrefix(s, "wechat")
}

// 密码验证: pwdVerify  同PHP函数 password_verify()
func PasswordVerify(pwd, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd))

	return err == nil
}

func JsonEncode(value interface{}) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func JsonDecode(src string, dest interface{}) error {
	return json.Unmarshal([]byte(src), dest)
}

func InterfaceToString(value interface{}) string {
	if str, ok := value.(string); ok {
		return str
	}
	return JsonEncode(value)
}

// CutWords 截取前 N 个单词
func CutWords(str string, num int) string {
	// 按空格分割字符串为单词切片
	words := strings.Fields(str)

	// 如果单词数量超过指定数量，则裁剪单词；否则保持原样
	if len(words) > num {
		return strings.Join(words[:num], " ") + " ..."
	} else {
		return str
	}
}

// HasChinese 判断文本是否含有中文
func HasChinese(text string) bool {
	for _, char := range text {
		if unicode.Is(unicode.Scripts["Han"], char) {
			return true
		}
	}
	return false
}

func StringToUint(s string) uint {
	// 先转换为 uint64
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	// 再转换为 uint
	return uint(val)
}

// NormalizeResolution 标准化视频分辨率输入
func NormalizeResolution(resolution string) string {
	switch strings.TrimSpace(resolution) {
	case "best", "720p", "1080p", "1440p", "2160p", "4k":
		return strings.TrimSpace(resolution)
	default:
		return "best"
	}
}
