package env

import (
	"os"
	"strconv"
	"strings"
)

// All gym env vars use GYM_ prefix to avoid collisions with other projects (e.g. worklist).

func DatabaseURL() string {
	if s := os.Getenv("GYM_DATABASE_URL"); s != "" {
		return s
	}
	// On Fly.io, use DATABASE_URL (set by postgres attach)
	if os.Getenv("FLY_APP_NAME") != "" {
		return os.Getenv("DATABASE_URL")
	}
	return ""
}

func GoogleClientID() string {
	return os.Getenv("GYM_GOOGLE_CLIENT_ID")
}

func Port() string {
	if s := os.Getenv("GYM_PORT"); s != "" {
		return s
	}
	return os.Getenv("PORT") // Fly sets this
}

// TLSCertFile returns the path to the TLS certificate file. When set with TLSKeyFile, server uses HTTPS.
func TLSCertFile() string { return os.Getenv("GYM_TLS_CERT_FILE") }

// TLSKeyFile returns the path to the TLS private key file. When set with TLSCertFile, server uses HTTPS.
func TLSKeyFile() string { return os.Getenv("GYM_TLS_KEY_FILE") }

func R2AccountID() string   { return os.Getenv("GYM_R2_ACCOUNT_ID") }
func R2AccessKeyID() string { return os.Getenv("GYM_R2_ACCESS_KEY_ID") }
func R2SecretAccessKey() string { return os.Getenv("GYM_R2_SECRET_ACCESS_KEY") }
func R2Bucket() string      { return os.Getenv("GYM_R2_BUCKET") }

func FCMCredentialsPath() string { return os.Getenv("GYM_FCM_CREDENTIALS_PATH") }

func OpenAIAPIKey() string { return os.Getenv("GYM_OPENAI_API_KEY") }
func OpenAITestMode() bool {
	return strings.ToLower(os.Getenv("GYM_OPENAI_TEST_MODE")) == "true"
}
func OpenAIRatePerMinute() int { return OpenAIEnvInt("GYM_OPENAI_RATE_PER_MINUTE", 10) }
func OpenAIDailyLimit() int    { return OpenAIEnvInt("GYM_OPENAI_DAILY_LIMIT", 100) }
func OpenAIDalleDailyLimit() int { return OpenAIEnvInt("GYM_OPENAI_DALLE_DAILY_LIMIT", 5) }

// DevMode enables dev token endpoint and Bearer dev:<email> auth. Never enable in production.
func DevMode() bool {
	return strings.ToLower(os.Getenv("GYM_DEV_MODE")) == "true"
}

// buildDate is set at build time via -ldflags "-X ...buildDate=..."
var buildDate string

// BuildDate returns the build timestamp. Empty when built with go run (no ldflags).
func BuildDate() string {
	if buildDate == "" {
		return "dev"
	}
	return buildDate
}

func OpenAIEnvInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}
