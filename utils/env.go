package utils

import (
	"github.com/joho/godotenv"
	"os"
)

var (
	Env                    = os.Getenv("ENV")
	Env_TracingServiceName = os.Getenv("TRACING_SERVICE_NAME")
	Env_OLTPEndpoint       = os.Getenv("OLTP_ENDPOINT")
)

func LoadEnv() {
	if _, err := os.Stat(".env"); err == nil {
		err = godotenv.Load()
		if err != nil {
			panic(err)
		}
	}
}
