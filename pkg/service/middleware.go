package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTelegramAuthTTL = 24 * time.Hour
	telegramAuthSkew       = time.Minute
	webAppSecretKey        = "WebAppData"
)

func telegramAuthTTL() time.Duration {
	hoursStr := os.Getenv("TELEGRAM_AUTH_TTL_HOURS")
	if hoursStr == "" {
		return defaultTelegramAuthTTL
	}
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		return defaultTelegramAuthTTL
	}
	return time.Duration(hours) * time.Hour
}

func webAppSecret(botToken string) []byte {
	mac := hmac.New(sha256.New, []byte(webAppSecretKey))
	mac.Write([]byte(botToken))
	return mac.Sum(nil)
}

func loginWidgetSecret(botToken string) []byte {
	sum := sha256.Sum256([]byte(botToken))
	return sum[:]
}

func verifyWebAppInitData(initData, botToken string) (map[string]string, error) {
	parsed, err := url.ParseQuery(initData)
	if err != nil {
		return nil, err
	}

	hash := parsed.Get("hash")
	parsed.Del("hash")

	fields := make(map[string]string, len(parsed))
	for key := range parsed {
		fields[key] = parsed.Get(key)
	}
	if err := verifySignedFields(fields, hash, webAppSecret(botToken)); err != nil {
		return nil, err
	}
	return fields, nil
}

func verifyLoginWidget(fields map[string]string, hash, botToken string) error {
	return verifySignedFields(fields, hash, loginWidgetSecret(botToken))
}

func verifySignedFields(fields map[string]string, hash string, secret []byte) error {
	if hash == "" {
		return errors.New("missing hash")
	}

	var keys []string
	for key := range fields {
		if key == "hash" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var checkStrings []string
	for _, key := range keys {
		checkStrings = append(checkStrings, key+"="+fields[key])
	}
	checkString := strings.Join(checkStrings, "\n")

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(checkString))
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(hash)
	if err != nil || !hmac.Equal(expected, got) {
		return errors.New("invalid Telegram auth")
	}
	return validateAuthDate(fields["auth_date"])
}

func validateAuthDate(authDateStr string) error {
	if authDateStr == "" {
		return errors.New("missing auth_date")
	}
	authDateUnix, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return errors.New("invalid auth_date")
	}
	authDate := time.Unix(authDateUnix, 0)
	if time.Since(authDate) > telegramAuthTTL() {
		return fmt.Errorf("initData expired (auth_date older than %s)", telegramAuthTTL())
	}
	if authDate.After(time.Now().Add(telegramAuthSkew)) {
		return errors.New("auth_date is in the future")
	}
	return nil
}
