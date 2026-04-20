// Package config provides centralized, environment-based configuration loading
// for all microservices. Implements Twelve-Factor App Factor III (Config).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServiceConfig contains common configuration for all microservices.
type ServiceConfig struct {
	// Service identity
	ServiceName    string
	ServiceVersion string
	Environment    string // development, staging, production
	GRPCPort       string
	HTTPPort       string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBMaxConns int
	DBTimeout  time.Duration

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// Kafka
	KafkaBrokers  string
	KafkaGroupID  string

	// Observability
	OTLPEndpoint string
	LogLevel     string
	TraceSampleRate float64

	// Security
	JWTPublicKeyPath string
	JWTIssuer        string

	// SMTP (Notification Service)
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
}

// Load populates configuration from environment variables.
// Every config value is sourced from the environment per 12-factor methodology.
// Defaults are provided for development; production values injected via
// Kubernetes ConfigMaps/Secrets or HashiCorp Vault.
func Load(serviceName string) *ServiceConfig {
	cfg := &ServiceConfig{
		ServiceName:    serviceName,
		ServiceVersion: getEnv("SERVICE_VERSION", "1.0.0"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		GRPCPort:       getEnv("GRPC_PORT", "50051"),
		HTTPPort:       getEnv("HTTP_PORT", "8080"),

		// Database defaults for Docker Compose development
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", serviceName+"_db"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),
		DBMaxConns: getEnvInt("DB_MAX_CONNS", 25),
		DBTimeout:  time.Duration(getEnvInt("DB_TIMEOUT_MS", 5000)) * time.Millisecond,

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		// Kafka
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", serviceName+"-group"),

		// Observability
		OTLPEndpoint:    getEnv("OTLP_ENDPOINT", "localhost:4317"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		TraceSampleRate: getEnvFloat("TRACE_SAMPLE_RATE", 1.0),

		// Security
		JWTPublicKeyPath: getEnv("JWT_PUBLIC_KEY_PATH", ""),
		JWTIssuer:        getEnv("JWT_ISSUER", "digital-metro-auth"),

		// SMTP
		SMTPHost: getEnv("SMTP_HOST", ""),
		SMTPPort: getEnv("SMTP_PORT", "587"),
		SMTPUser: getEnv("SMTP_USER", ""),
		SMTPPass: getEnv("SMTP_PASS", ""),
	}

	return cfg
}

// DSN returns the PostgreSQL connection string.
func (c *ServiceConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

// RedisAddr returns the Redis connection address.
func (c *ServiceConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

// GRPCListenAddr returns the gRPC listen address.
func (c *ServiceConfig) GRPCListenAddr() string {
	return ":" + c.GRPCPort
}

// HTTPListenAddr returns the HTTP listen address.
func (c *ServiceConfig) HTTPListenAddr() string {
	return ":" + c.HTTPPort
}

// IsProduction returns true if running in production environment.
func (c *ServiceConfig) IsProduction() bool {
	return c.Environment == "production"
}

// ---- Helpers ----

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}
