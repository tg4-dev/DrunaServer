package repository

import (
	"net"
	"net/url"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	usersTable   = "users"
	eventsTable  = "events"
	friendsTable = "friends"
	groupTable   = "groups"
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	DBName   string
	SSLMode  string
}

func NewPostgresDB(cfg Config) (*sqlx.DB, error) {
	dsnURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   "/" + cfg.DBName,
	}
	query := url.Values{}
	query.Set("sslmode", cfg.SSLMode)
	dsnURL.RawQuery = query.Encode()
	dsn := dsnURL.String()

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
