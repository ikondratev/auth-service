package settings

import "errors"

var (
	ErrReadFile           = errors.New("settings: open file error")
	ErrFileJsonOpen       = errors.New("settings: parse file error")
	ErrValiationEnv       = errors.New("settings: validation error env key")
	ErrValidationGRPC     = errors.New("settings: validation error GRPC key")
	ErrValidationGRPCPort = errors.New("settings: validation error GRPC Port key")
)
