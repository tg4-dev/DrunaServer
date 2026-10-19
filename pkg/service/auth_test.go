package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"os"
	"testing"
	"time"
)

type mockAuthRepo struct {
	users map[string]model.User
}

func (m *mockAuthRepo) CreateUser(user model.User) (int, error) {
	user.ID = len(m.users) + 1
	m.users[user.Username] = user
	return user.ID, nil
}

func (m *mockAuthRepo) GetUserByUsername(username string) (model.User, error) {
	user, ok := m.users[username]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (m *mockAuthRepo) GetUserByID(userID int) (model.User, error) {
	for _, user := range m.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (m *mockAuthRepo) UpdateUserProfile(userID int, name, avatarURL string) error {
	for username, user := range m.users {
		if user.ID == userID {
			if name != "" {
				user.Name = name
			}
			if avatarURL != "" {
				user.AvatarURL = avatarURL
			}
			m.users[username] = user
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *mockAuthRepo) SearchUsers(prefix string) ([]model.FriendInfo, error) {
	return []model.FriendInfo{}, nil
}

type mockTokenRepo struct {
	claimed  map[string]string
	families map[string]bool
}

func (m *mockTokenRepo) ClaimRefreshToken(jti, familyID string, expiresAt time.Time) (bool, error) {
	if m.claimed == nil {
		m.claimed = map[string]string{}
	}
	if _, ok := m.claimed[jti]; ok {
		return false, nil
	}
	m.claimed[jti] = familyID
	return true, nil
}

func (m *mockTokenRepo) RevokeFamily(familyID string, expiresAt time.Time) error {
	if m.families == nil {
		m.families = map[string]bool{}
	}
	m.families[familyID] = true
	return nil
}

func (m *mockTokenRepo) IsFamilyRevoked(familyID string) (bool, error) {
	return m.families != nil && m.families[familyID], nil
}

func (m *mockTokenRepo) Ping() error { return nil }

func (m *mockTokenRepo) PurgeExpiredTokens() (int64, error) { return 0, nil }

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-only")
	os.Exit(m.Run())
}

func TestCreateUserPasswordValidation(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{users: map[string]model.User{}}, &mockTokenRepo{})
	_, err := svc.CreateUser(model.User{
		Name:     "Short",
		Username: "short",
		Email:    "short@test.local",
		Password: "abc",
	})
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := hashPassword("secret123")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if hash == "secret123" {
		t.Fatal("password must be hashed")
	}
}

func TestGenerateAndParseAccessToken(t *testing.T) {
	hash, _ := hashPassword("password")
	repo := &mockAuthRepo{
		users: map[string]model.User{
			"alice": {ID: 42, Username: "alice", PasswordHash: hash},
		},
	}
	svc := NewAuthService(repo, &mockTokenRepo{})

	access, refresh, err := svc.GenerateAccessRefreshToken("alice", "password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	userID, username, err := svc.ParseAccessToken(access)
	if err != nil {
		t.Fatalf("parse access token failed: %v", err)
	}
	if userID != 42 || username != "alice" {
		t.Fatalf("unexpected claims: %d %s", userID, username)
	}

	_, _, err = svc.ParseAccessToken(refresh)
	if err == nil {
		t.Fatal("refresh token must not parse as access token")
	}
}

func TestRenewTokenRotatesRefresh(t *testing.T) {
	hash, _ := hashPassword("password")
	repo := &mockAuthRepo{
		users: map[string]model.User{
			"bob": {ID: 1, Username: "bob", PasswordHash: hash},
		},
	}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(repo, tokenRepo)

	_, refresh, err := svc.GenerateAccessRefreshToken("bob", "password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	newAccess, newRefresh, err := svc.RenewToken(refresh)
	if err != nil {
		t.Fatalf("renew failed: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Fatal("expected new tokens")
	}

	_, _, err = svc.RenewToken(refresh)
	if err == nil {
		t.Fatal("old refresh token should be revoked")
	}
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	hash, _ := hashPassword("password")
	repo := &mockAuthRepo{
		users: map[string]model.User{
			"cara": {ID: 7, Username: "cara", PasswordHash: hash},
		},
	}
	tokenRepo := &mockTokenRepo{}
	svc := NewAuthService(repo, tokenRepo)

	_, refresh, err := svc.GenerateAccessRefreshToken("cara", "password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_, rotated, err := svc.RenewToken(refresh)
	if err != nil {
		t.Fatalf("renew failed: %v", err)
	}
	if _, _, err := svc.RenewToken(refresh); err == nil {
		t.Fatal("expected reused refresh to fail")
	}
	if _, _, err := svc.RenewToken(rotated); err == nil {
		t.Fatal("expected family revocation to reject the rotated refresh token")
	}
}

func TestSignInHidesUnknownUser(t *testing.T) {
	hash, _ := hashPassword("password")
	repo := &mockAuthRepo{
		users: map[string]model.User{
			"dana": {ID: 3, Username: "dana", PasswordHash: hash},
		},
	}
	svc := NewAuthService(repo, &mockTokenRepo{})

	_, _, missingErr := svc.GenerateAccessRefreshToken("nobody", "password")
	_, _, badPassErr := svc.GenerateAccessRefreshToken("dana", "wrong-password")
	if !errors.Is(missingErr, ErrInvalidCredentials) || !errors.Is(badPassErr, ErrInvalidCredentials) {
		t.Fatalf("expected identical invalid credentials errors, missing=%v bad=%v", missingErr, badPassErr)
	}
	if missingErr.Error() != badPassErr.Error() {
		t.Fatalf("error text differs: %q vs %q", missingErr.Error(), badPassErr.Error())
	}
}

func TestCreateUserIgnoresAvatar(t *testing.T) {
	repo := &mockAuthRepo{users: map[string]model.User{}}
	svc := NewAuthService(repo, &mockTokenRepo{})
	_, err := svc.CreateUser(model.User{
		Name:      "Eve",
		Username:  "eve",
		Email:     "eve@test.local",
		Password:  "secret12345",
		AvatarURL: "https://example.com/a.png",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if stored := repo.users["eve"]; stored.AvatarURL != "" {
		t.Fatalf("avatar must not be stored on password signup: %s", stored.AvatarURL)
	}
}

func TestCreateUserRejectsLongPassword(t *testing.T) {
	svc := NewAuthService(&mockAuthRepo{users: map[string]model.User{}}, &mockTokenRepo{})
	_, err := svc.CreateUser(model.User{
		Name:     "Long",
		Username: "longpw",
		Email:    "long@test.local",
		Password: string(make([]byte, 73)),
	})
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}
