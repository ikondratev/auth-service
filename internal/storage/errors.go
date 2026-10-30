package storage

import "errors"

var (
	ErrOpenConnection  = errors.New("storage: open connection error")
	ErrCloseConnection = errors.New("storage: close connection error")
)
