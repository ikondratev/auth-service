package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
)

const (
	settingPath = "settings"
	fileNam     = "settings.json"
)

type Settings struct {
	Env    string `json:"env" validate:"required"`
	Server Server `json:"server" validate:"required"`
	GRPC   GRPC   `json:"grpc" validate:"required"`
	DB     Db     `json:"db" validate:"required"`
	Auth   Auth   `json:"auth" validate:"required"`
}

type Server struct {
	WaitingShutdownSec int `json:"waiting_shutdown_second" validate:"required,min=1"`
}

type GRPC struct {
	Port string `json:"port" validate:"required"`
}

type Db struct {
	Url               string `json:"url" validate:"required"`
	OpenConnection    int    `json:"open_conn" validate:"required,min=1"`
	MaxIdleConnection int    `json:"max_idle_conn" validate:"required,min=1"`
	MaxLifeTime       int    `json:"max_life_time" validate:"required"`
	StartDelay        int    `json:"start_delay" validate:"required"`
}

type Auth struct {
	Issuer         string `json:"issuer" validate:"required"`
	Audience       string `json:"audience" validate:"required"`
	PrivateKeyPath string `json:"private_key_path" validate:"required"`
	AccessTTLSec   int    `json:"access_ttl_sec" validate:"required,min=1"`
	RefreshTTLSec  int    `json:"refresh_ttl_sec" validate:"required,min=1"`
	Scope          string `json:"scope" validate:"required"`
}

func New(e string) (*Settings, error) {
	var s Settings
	filePath := fmt.Sprintf("%s/%s.%s", settingPath, e, fileNam)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, ErrReadFile
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return nil, ErrFileJsonOpen
	}

	if err := validate(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func validate(s *Settings) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(s); err != nil {
		return errors.Join(ErrValiation, fmt.Errorf(":%s", err))
	}

	return nil
}
