package api

import (
	"math/rand"
	"sync"
	"time"
)

var (
	TokenManager = tokenManager{
		tokens: make(map[string]*UserSession),
	}
)

type UserSession struct {
	Username  string
	Pwd       string
	LoginTime time.Time
	AliveTime time.Time
}

type tokenManager struct {
	tokens map[string]*UserSession

	lock sync.RWMutex
}

func (t *tokenManager) Add(token string, username string, pwd string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.tokens[token] = &UserSession{
		Username:  username,
		Pwd:       pwd,
		LoginTime: time.Now(),
		AliveTime: time.Now(),
	}
}

func (t *tokenManager) Find(token string) *UserSession {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.tokens[token]
}

func (t *tokenManager) Remove(token string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.tokens, token)
}

func (t *tokenManager) Refresh(token string, time2 time.Time) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	session, ok := t.tokens[token]
	if !ok {
		return false
	}

	session.AliveTime = time2
	return true
}

func (t *tokenManager) Start(timeout time.Duration) {
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			t.lock.Lock()
			for token, session := range t.tokens {
				if time.Since(session.AliveTime) > timeout {
					delete(t.tokens, token)
				}
			}
			t.lock.Unlock()
		}
	}
}

func (t *tokenManager) Clear() {
	// 清空所有token
	t.lock.Lock()
	defer t.lock.Unlock()
	t.tokens = make(map[string]*UserSession)
}

// GenerateToken 生成token
func GenerateToken() string {
	// 从大小写字母和数字中随机选择
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// 随机选择16个字符
	token := make([]byte, 16)
	for i := range token {
		token[i] = charset[rand.Intn(len(charset))]
	}
	return string(token)
}
