package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv       string
	AppPort      string
	BaseURL      string
	CORSOrigins  string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	DBTimezone   string
	JWTSecret    string
	JWTTTL       time.Duration
	UploadDir    string
	MaxUploadMB  int
	GeminiAPIKey string
	GeminiModel  string
	AutoMigrate  bool
	SeedData     bool
}

func Load() *Config {
	_ = godotenv.Load()
	ttlHours, _ := strconv.Atoi(env("JWT_TTL_HOURS", "72"))
	maxMB, _ := strconv.Atoi(env("MAX_UPLOAD_MB", "5"))
	port := env("APP_PORT", "8080")
	return &Config{
		AppEnv:       env("APP_ENV", "development"),
		AppPort:      port,
		BaseURL:      env("BASE_URL", "http://localhost:"+port),
		CORSOrigins:  env("CORS_ORIGINS", "*"),
		DBHost:       env("DB_HOST", "localhost"),
		DBPort:       env("DB_PORT", "5432"),
		DBUser:       env("DB_USER", "temuin"),
		DBPassword:   env("DB_PASSWORD", "temuin"),
		DBName:       env("DB_NAME", "temuin"),
		DBSSLMode:    env("DB_SSLMODE", "disable"),
		DBTimezone:   env("DB_TIMEZONE", "Asia/Jakarta"),
		JWTSecret:    env("JWT_SECRET", "change-me-in-production"),
		JWTTTL:       time.Duration(ttlHours) * time.Hour,
		UploadDir:    env("UPLOAD_DIR", "./uploads"),
		MaxUploadMB:  maxMB,
		GeminiAPIKey: env("GEMINI_API_KEY", ""),
		GeminiModel:  env("GEMINI_MODEL", "gemini-2.5-flash"),
		AutoMigrate:  env("AUTO_MIGRATE", "true") == "true",
		SeedData:     env("SEED_DATA", "true") == "true",
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode, c.DBTimezone)
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}
