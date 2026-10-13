package service

type ErrorKind int

const (
	KindValidation ErrorKind = iota
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
)

// DomainError is a client-safe error. Handlers map Kind to an HTTP status
// and return Msg. Unknown errors stay as 500 without the underlying text.
type DomainError struct {
	Kind ErrorKind
	Msg  string
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func Validation(msg string) *DomainError {
	return &DomainError{Kind: KindValidation, Msg: msg}
}

func Unauthorized(msg string) *DomainError {
	return &DomainError{Kind: KindUnauthorized, Msg: msg}
}

func Forbidden(msg string) *DomainError {
	return &DomainError{Kind: KindForbidden, Msg: msg}
}

func NotFound(msg string) *DomainError {
	return &DomainError{Kind: KindNotFound, Msg: msg}
}

func Conflict(msg string) *DomainError {
	return &DomainError{Kind: KindConflict, Msg: msg}
}

var (
	ErrPasswordTooShort    = Validation("password must be at least 8 characters")
	ErrPasswordTooLong     = Validation("password must be at most 72 bytes")
	ErrInvalidCredentials  = Unauthorized("invalid credentials")
	ErrGroupAccessDenied   = Forbidden("you are not a member of this group")
	ErrGroupEventNotFound  = NotFound("group event not found")
	ErrGroupEventForbidden = Forbidden("only the event creator or group owner can modify this event")
)
