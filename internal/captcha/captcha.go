package captcha

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	digitLen = 4
	ttl      = 5 * time.Minute
)

type entry struct {
	code    string
	expires time.Time
}

var (
	mu    sync.Mutex
	store = make(map[string]entry)
)

// randomDigits returns a string of n decimal digits.
func randomDigits(n int) string {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		x, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			b[i] = digits[time.Now().UnixNano()%10]
			continue
		}
		b[i] = digits[x.Int64()]
	}
	return string(b)
}

func cleanupLocked() {
	now := time.Now()
	for t, e := range store {
		if now.After(e.expires) {
			delete(store, t)
		}
	}
}

// Create generates a new numeric captcha, returns token and a data: URL (SVG).
func Create() (token string, dataURL string) {
	code := randomDigits(digitLen)
	token = uuid.New().String()
	mu.Lock()
	cleanupLocked()
	store[token] = entry{code: code, expires: time.Now().Add(ttl)}
	mu.Unlock()

	svg := renderSVG(code)
	b64 := base64.StdEncoding.EncodeToString([]byte(svg))
	return token, "data:image/svg+xml;base64," + b64
}

// Verify checks the user input against the stored code (single use). Input is trimmed;
// comparison is digit-only exact match.
func Verify(token, userInput string) (ok bool, errMsg string) {
	userInput = strings.TrimSpace(userInput)
	if token == "" || userInput == "" {
		return false, "请输入验证码"
	}
	if len(userInput) != digitLen {
		return false, "验证码须为 4 位数字"
	}
	for i := 0; i < len(userInput); i++ {
		if userInput[i] < '0' || userInput[i] > '9' {
			return false, "验证码须为 4 位数字"
		}
	}
	mu.Lock()
	defer mu.Unlock()
	e, found := store[token]
	if !found || time.Now().After(e.expires) {
		return false, "验证码已过期，请点击图片刷新"
	}
	if e.code != userInput {
		return false, "验证码错误"
	}
	delete(store, token)
	return true, ""
}

// renderSVG draws a simple 4-digit captcha (readable, light noise).
func renderSVG(code string) string {
	if len(code) != digitLen {
		code = "0000"
	}
	w, h := 140, 50
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, w, h)
	fmt.Fprintf(&b, `<rect fill="#f4f4f5" width="100%%" height="100%%" rx="6"/>`)
	for i := 0; i < 5; i++ {
		x1, y1 := randIntn(w), randIntn(h)
		x2, y2 := randIntn(w), randIntn(h)
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#d4d4d8" stroke-width="1" opacity="0.7"/>`, x1, y1, x2, y2)
	}
	xs := []int{20, 50, 80, 110}
	for i, ch := range code {
		rot := (i*5 - 6) + randIntn(9) - 4
		d := html.EscapeString(string(ch))
		fmt.Fprintf(&b, `<text x="%d" y="34" font-size="24" font-weight="600" font-family="ui-monospace,Courier New,monospace" fill="#3f3f46" transform="rotate(%d %d 28)">%s</text>`,
			xs[i], rot, xs[i], d)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func randIntn(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(max))
	}
	return int(n.Int64())
}
