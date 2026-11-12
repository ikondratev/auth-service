package auth

import "errors"

const alreadyRegisterErrCode = "23505"

var (
	ErrInvalidEmail       = errors.New("auth: invalid Email")
	ErrInvalidPassword    = errors.New("auth: invalid Password")
	ErrEmailTaken         = errors.New("auth: Email already registred")
	ErrToken              = errors.New("auth: token erorr")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrInvalidRefresh     = errors.New("auth: invalid refresh token")
)
