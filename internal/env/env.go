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
	return os.Getenv("DATABASE_URL") // Fly postgres attach sets this
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

func OpenAIEnvInt(key string, def int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}
