package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signFields(t *testing.T, fields map[string]string, secret []byte) string {
	t.Helper()
	var keys []string
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(mac.Sum(nil))
}

func sampleFields() map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":123,"first_name":"Test","username":"tester"}`,
	}
}

func TestVerifyWebAppInitDataValid(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := sampleFields()
	fields["hash"] = signFields(t, fields, webAppSecret(botToken))

	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	result, err := verifyWebAppInitData(values.Encode(), botToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["user"] == "" {
		t.Fatal("expected user in parsed data")
	}
}

func TestVerifyLoginWidgetValid(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := map[string]string{
		"auth_date":  strconv.FormatInt(time.Now().Unix(), 10),
		"first_name": "Test",
		"id":         "123",
		"username":   "tester",
	}
	hash := signFields(t, fields, loginWidgetSecret(botToken))
	if err := verifyLoginWidget(fields, hash, botToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebAppSignatureRejectedByWidgetVerifier(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := sampleFields()
	hash := signFields(t, fields, webAppSecret(botToken))
	if err := verifyLoginWidget(fields, hash, botToken); err == nil {
		t.Fatal("widget verifier must reject a Mini App signature")
	}
}

func TestWidgetSignatureRejectedByWebAppVerifier(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := sampleFields()
	hash := signFields(t, fields, loginWidgetSecret(botToken))
	fields["hash"] = hash
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	if _, err := verifyWebAppInitData(values.Encode(), botToken); err == nil {
		t.Fatal("webapp verifier must reject a Login Widget signature")
	}
}

func TestVerifyWebAppInvalidHash(t *testing.T) {
	_, err := verifyWebAppInitData("user=%7B%7D&hash=bad", "token")
	if err == nil {
		t.Fatal("expected invalid hash error")
	}
}

func TestVerifyWebAppMissingHash(t *testing.T) {
	_, err := verifyWebAppInitData("user=%7B%7D", "token")
	if err == nil {
		t.Fatal("expected missing hash error")
	}
}

func TestVerifyWebAppExpired(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := sampleFields()
	fields["auth_date"] = strconv.FormatInt(time.Now().Add(-48*time.Hour).Unix(), 10)
	fields["hash"] = signFields(t, map[string]string{
		"auth_date": fields["auth_date"],
		"user":      fields["user"],
	}, webAppSecret(botToken))
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	if _, err := verifyWebAppInitData(values.Encode(), botToken); err == nil {
		t.Fatal("expected expired initData error")
	}
}

func TestVerifyWebAppFutureAuthDate(t *testing.T) {
	botToken := "123456:ABC-DEF"
	fields := sampleFields()
	fields["auth_date"] = strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)
	signed := map[string]string{
		"auth_date": fields["auth_date"],
		"user":      fields["user"],
	}
	fields["hash"] = signFields(t, signed, webAppSecret(botToken))
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	if _, err := verifyWebAppInitData(values.Encode(), botToken); err == nil {
		t.Fatal("expected future auth_date error")
	}
}
