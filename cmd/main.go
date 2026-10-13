package main

import (
	"context"
	druna "druna_server"
	"druna_server/pkg/handler"
	"druna_server/pkg/repository"
	"druna_server/pkg/service"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// @title Druna API
// @version 1.0
// @description API server for Druna App

//@host localhost:8000
//@BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	logrus.SetFormatter(new(logrus.JSONFormatter))

	if err := initConfig(); err != nil {
		logrus.Fatalf("Error initializing config configs: %s", err)
	}

	if err := godotenv.Load(); err != nil {
		logrus.Warnf("No .env file loaded: %s", err)
	}

	if os.Getenv("JWT_SECRET") == "" {
		logrus.Fatal("JWT_SECRET environment variable is required")
	}

	db, err := repository.NewPostgresDB(repository.Config{
		Host:     viper.GetString("db.host"),
		Port:     viper.GetString("db.port"),
		Username: viper.GetString("db.username"),
		DBName:   viper.GetString("db.dbname"),
		SSLMode:  viper.GetString("db.sslmode"),
		Password: os.Getenv("DB_PASSWORD"),
	})

	if err != nil {
		logrus.Fatalf("Failed to init DB: %s", err.Error())
	}

	repos := repository.NewRepository(db)
	services := service.NewService(repos)
	handlers := handler.NewHandler(services)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	startTokenPurge(ctx, repos.Token)

	srv := new(druna.Server)

	go func() {
		if err := srv.Run(viper.GetString("port"), handlers.InitRoutes()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Fatalf("Error occured while running http server: %s", err)
		}
	}()

	logrus.Println("DrunaServer started succesfully")

	<-ctx.Done()

	logrus.Println("DrunaServer shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("error occured on server shutdown: %v", err)
	}
	if err := db.Close(); err != nil {
		logrus.Errorf("error occured on db closing: %v", err)
	}
}

func initConfig() error {
	viper.AddConfigPath("configs")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	return viper.ReadInConfig()
}

func startTokenPurge(ctx context.Context, tokenRepo repository.Token) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := tokenRepo.PurgeExpiredTokens()
				if err != nil {
					logrus.WithError(err).Error("failed to purge expired revoked tokens")
					continue
				}
				if count > 0 {
					logrus.WithField("count", count).Info("purged expired revoked tokens")
				}
			}
		}
	}()
}
