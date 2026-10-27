package main

import (
	"os"

	"github.com/ikondratev/auth-service/internal/application"
)

func main() {
	app, err := application.New(getEnv())
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func getEnv() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	return env
}
