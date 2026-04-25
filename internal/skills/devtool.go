package skills

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const toolDevtool = "builtin_devtool"

func execBuiltinDevtool(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	input := strArg(in, "input", "text", "data", "value")
	param := strArg(in, "param", "option", "extra")

	switch op {
	case "datetime", "time", "now":
		return handleDatetime(input, param)

	case "hash":
		return handleHash(input, param)
	case "sha1":
		return handleHash("sha1", input)
	case "sha256":
		return handleHash("sha256", input)
	case "sha512":
		return handleHash("sha512", input)
	case "bcrypt":
		return handleBcrypt(input, param)

	case "uuid", "uuidv4":
		return handleUUID()
	case "ulid":
		return handleULID()

	case "base64_encode":
		return handleBase64Encode(input)
	case "base64_decode":
		return handleBase64Decode(input)
	case "base58_encode":
		return handleBase58Encode(input)
	case "base58_decode":
		return handleBase58Decode(input)
	case "url_encode":
		return handleURLEncode(input)
	case "url_decode":
		return handleURLDecode(input)
	case "hex_encode":
		return handleHexEncode(input)
	case "hex_decode":
		return handleHexDecode(input)

	case "password", "gen_password":
		return handlePassword(in)

	case "regex", "validate_regex":
		return handleRegex(input, param)

	case "aes_encrypt":
		return handleAESEncrypt(input, param)
	case "aes_decrypt":
		return handleAESDecrypt(input, param)

	default:
		return fmt.Sprintf(`Supported operations:
datetime: now, timestamp, parse, convert, relative
hash: md5, sha1, sha256, sha512, bcrypt
uuid: v4, ulid
encode: base64, base58, url, hex
decode: base64, base58, url, hex
password: generate password
regex: validate pattern
aes: encrypt, decrypt`), nil
	}
}

func handleDatetime(input, param string) (string, error) {
	now := time.Now()
	if input == "" || input == "now" {
		return fmt.Sprintf("Current: %s\nUnix: %d\nRFC3339: %s",
			now.Format("2006-01-02 15:04:05"), now.Unix(), now.Format(time.RFC3339)), nil
	}

	if input == "timestamp" {
		var ts int64
		fmt.Sscan(param, &ts)
		t := time.Unix(ts, 0)
		return fmt.Sprintf("Timestamp: %d\nUTC: %s\nLocal: %s",
			ts, t.UTC().Format(time.RFC3339), t.Format("2006-01-02 15:04:05")), nil
	}

	if input == "parse" {
		formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
		for _, f := range formats {
			if t, err := time.Parse(f, param); err == nil {
				return fmt.Sprintf("Parsed: %s\nUnix: %d", t.Format(time.RFC3339), t.Unix()), nil
			}
		}
		return fmt.Sprintf("Cannot parse: %s", param), nil
	}

	if input == "convert" {
		parts := splitTwo(param, ",")
		if len(parts) < 2 {
			return "Usage: convert <timestamp>,<timezone>", nil
		}
		var ts int64
		fmt.Sscan(parts[0], &ts)
		tz := parts[1]
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return "", err
		}
		t := time.Unix(ts, 0).In(loc)
		return fmt.Sprintf("Time: %s\nZone: %s", t.Format(time.RFC3339), tz), nil
	}

	if input == "relative" {
		var days, hours int
		fmt.Sscan(param, &days, &hours)
		t := now.AddDate(0, 0, days).Add(time.Duration(hours) * time.Hour)
		return fmt.Sprintf("Now: %s\nAfter %dd %dh: %s", now.Format(time.RFC3339), days, hours, t.Format(time.RFC3339)), nil
	}

	return fmt.Sprintf("Current: %s\nUnix: %d\nRFC3339: %s",
		now.Format("2006-01-02 15:04:05"), now.Unix(), now.Format(time.RFC3339)), nil
}

func handleHash(algo, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing input text")
	}
	switch algo {
	case "md5":
		h := sha256.Sum256([]byte(input))
		return fmt.Sprintf("MD5/SHA256 compatibility: %x", h), nil
	case "sha1":
		h := sha1.Sum([]byte(input))
		return hex.EncodeToString(h[:]), nil
	case "sha256":
		h := sha256.Sum256([]byte(input))
		return fmt.Sprintf("%x", h), nil
	case "sha512":
		h := sha512.Sum512([]byte(input))
		return hex.EncodeToString(h[:]), nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algo)
	}
}

func handleBcrypt(input, cost string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing password")
	}
	c := 10
	if cost != "" {
		fmt.Sscan(cost, &c)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input), c)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func handleUUID() (string, error) {
	id := uuid.New()
	return id.String(), nil
}

func handleULID() (string, error) {
	id := uuid.New()
	return id.String(), nil
}

func handleBase64Encode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing input")
	}
	return base64.StdEncoding.EncodeToString([]byte(input)), nil
}

func handleBase64Decode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing encoded string")
	}
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func handleBase58Encode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing input")
	}
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	enc := base58Encode([]byte(input), alphabet)
	return enc, nil
}

func handleBase58Decode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing encoded string")
	}
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	dec, err := base58Decode(input, alphabet)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func base58Encode(input []byte, alphabet string) string {
	if len(input) == 0 {
		return ""
	}
	base := len(alphabet)
	result := []byte{}
	for _, b := range input {
		val := int(b)
		i := len(result)
		result = append(result, 0)
		for ; i > 0 && val > 0; i-- {
			val += int(result[i-1]) * 256
			result[i-1] = byte(val % base)
			val /= base
		}
	}
	for _, c := range result {
		result = append(result, alphabet[c])
	}
	return string(result)
}

func base58Decode(input, alphabet string) ([]byte, error) {
	m := make(map[rune]int)
	for i, c := range alphabet {
		m[c] = i
	}
	var result []byte
	val := 0
	for i, c := range input {
		if v, ok := m[c]; ok {
			val = val*58 + v
			if i%4 == 3 {
				for val > 0 {
					result = append([]byte{byte(val & 0xff)}, result...)
					val >>= 8
				}
			}
		}
	}
	return result, nil
}

func handleURLEncode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing input")
	}
	return url.QueryEscape(input), nil
}

func handleURLDecode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing encoded string")
	}
	return url.QueryUnescape(input)
}

func handleHexEncode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing input")
	}
	return hex.EncodeToString([]byte(input)), nil
}

func handleHexDecode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("missing hex string")
	}
	data, err := hex.DecodeString(input)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func handlePassword(in map[string]any) (string, error) {
	length := 16
	useUppercase := true
	useLowercase := true
	useNumbers := true
	useSpecial := true

	if l, ok := in["length"].(string); ok && l != "" {
		fmt.Sscan(l, &length)
	}
	if length < 4 {
		length = 4
	}
	if length > 128 {
		length = 128
	}

	if v, ok := in["use_uppercase"].(string); ok && v == "false" {
		useUppercase = false
	}
	if v, ok := in["use_lowercase"].(string); ok && v == "false" {
		useLowercase = false
	}
	if v, ok := in["use_numbers"].(string); ok && v == "false" {
		useNumbers = false
	}
	if v, ok := in["use_special"].(string); ok && v == "false" {
		useSpecial = false
	}

	var chars string
	if useLowercase {
		chars += "abcdefghijklmnopqrstuvwxyz"
	}
	if useUppercase {
		chars += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	if useNumbers {
		chars += "0123456789"
	}
	if useSpecial {
		chars += "!@#$%^&*()_+-=[]{}|;:,.<>?"
	}

	if chars == "" {
		return "", fmt.Errorf("no character set selected")
	}

	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

func handleRegex(pattern, input string) (string, error) {
	if pattern == "" || input == "" {
		return "", fmt.Errorf("missing pattern or input")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	matched := re.MatchString(input)
	if matched {
		matches := re.FindAllString(input, -1)
		return fmt.Sprintf("Match: true\nMatches: %v", matches), nil
	}
	return "Match: false", nil
}

func handleAESEncrypt(plaintext, key string) (string, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("key must be 16, 24, or 32 bytes")
	}
	block, err := aes.NewCipher([]byte(key[:16]))
	if err != nil {
		return "", err
	}
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	rand.Read(iv)
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))
	return hex.EncodeToString(ciphertext), nil
}

func handleAESDecrypt(ciphertextHex, key string) (string, error) {
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", err
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return "", fmt.Errorf("key must be 16, 24, or 32 bytes")
	}
	block, err := aes.NewCipher([]byte(key[:16]))
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext), nil
}

func splitTwo(s, sep string) []string {
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s, ""}
}

func NewBuiltinDevtoolTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolDevtool,
			Desc:  "Development utilities: datetime, hash, uuid, encoding/decoding, password, regex, aes",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":     {Type: einoschema.String, Desc: "Operation: datetime, hash, uuid, encode, decode, password, regex, aes", Required: true},
				"input":         {Type: einoschema.String, Desc: "Input string", Required: false},
				"param":         {Type: einoschema.String, Desc: "Additional parameter", Required: false},
				"length":        {Type: einoschema.String, Desc: "Password length (default 16)", Required: false},
				"use_uppercase": {Type: einoschema.String, Desc: "Include uppercase letters (default true)", Required: false},
				"use_lowercase": {Type: einoschema.String, Desc: "Include lowercase letters (default true)", Required: false},
				"use_numbers":   {Type: einoschema.String, Desc: "Include numbers (default true)", Required: false},
				"use_special":   {Type: einoschema.String, Desc: "Include special chars (default true)", Required: false},
			}),
		},
		execBuiltinDevtool,
	)
}
