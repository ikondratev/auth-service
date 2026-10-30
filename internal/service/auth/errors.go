package auth

import "errors"

const alreadyRegisterErrCode = "23505"

var (
	ErrInvalidEmail    = errors.New("auth: invalid Email")
	ErrInvalidPassword = errors.New("auth: invalid Password")
	ErrEmailTaken      = errors.New("auth: Email already registred")
	ErrToken           = errors.New("auth: token erorr")
)
