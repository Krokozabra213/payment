package apperror

import (
	"errors"
	"fmt"
	"log/slog"
)

type Code int

const (
	CodeNotFound          Code = iota + 1 // 1
	CodeValidation                        // 2
	CodeAlreadyExists                     // 3
	CodeInsufficientFunds                 // 4
	CodeInternal                          // 5
	CodeConflict                          // 6
	CodeInvalidArgument                   // 7
	// CodeUnauthorized    Code = "UNAUTHORIZED"
	// CodeForbidden       Code = "FORBIDDEN"
	// CodeBadRequest        Code = "BAD_REQUEST"
)

type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

type AppError struct {
	code     Code
	message  string
	op       string // операция на которой возникла ошибка (user.create, repo.get, etc.)
	err      error  // оригинальная ошибка (которая пришла с репозитория)
	attrs    Fields // structured logging
	logLevel Level
}

func (e *AppError) Err() error {
	return e.err
}

func (e *AppError) Code() Code {
	return e.code
}

// Message возвращает сообщение об ошибке
func (e *AppError) Message() string {
	return e.message
}

// Op возвращает операцию, на которой возникла ошибка
func (e *AppError) Op() string {
	return e.op
}

// Attrs возвращает структурированные атрибуты для логирования
func (e *AppError) Attrs() Fields {
	return e.attrs
}

// LogLevel возвращает уровень логирования
func (e *AppError) LogLevel() Level {
	return e.logLevel
}

func (e *AppError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.op, e.message, e.err)
	}
	return fmt.Sprintf("[%s] %s", e.op, e.message)
}

func (e *AppError) Unwrap() error {
	return e.err
}

func NewAppErr(code Code, op, message string, err error, level Level, fields Fields) *AppError {
	return &AppError{
		code:     code,
		op:       op,
		message:  message,
		err:      err,
		attrs:    fields,
		logLevel: level,
	}
}

func (e *AppError) AddFields(fields ...Field) *AppError {
	if e == nil {
		return nil
	}
	newErr := *e
	newErr.attrs = make(Fields, len(e.attrs), len(e.attrs)+len(fields))
	copy(newErr.attrs, e.attrs)
	newErr.attrs = append(newErr.attrs, fields...)
	return &newErr
}

func GetAppErr(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

func SlogLevelFromAppLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
