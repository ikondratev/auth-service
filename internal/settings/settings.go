package settings

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	settingPath = "settings"
	fileNam     = "settings.json"
)

type Settings struct {
	Env    string `json:"env"`
	Server Server `json:"server"`
	GRPC   GRPC   `json:"grpc"`
}

type Server struct {
	WaitingShutdownSec int `json:"waiting_shutdown_second"`
}

type GRPC struct {
	Port string `json:"port"`
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
	if s.Env == "" {
		return ErrValiationEnv
	}

	var zero GRPC
	if s.GRPC == zero {
		return ErrValidationGRPC
	}

	if s.GRPC.Port == "" {
		return ErrValidationGRPCPort
	}

	return nil
}
