package fault

import (
	"errors"
	"fmt"
)

type Error struct {
	Status        int
	Code, Message string
	Fields        map[string]string
}

func (e *Error) Error() string { return e.Message }
func New(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}
func Validation(message string) error { return New(422, "validation_failed", message) }
func NotFound() error                 { return New(404, "not_found", "Nicht gefunden") }
func Conflict() error {
	return New(409, "version_conflict", "Zwischenzeitlich geändert. Bitte neu laden.")
}
func Wrap(err error) *Error {
	var f *Error
	if errors.As(err, &f) {
		return f
	}
	return &Error{Status: 500, Code: "internal_error", Message: "Vorgang fehlgeschlagen"}
}
func Field(id string, err error) error {
	return &Error{Status: 422, Code: "validation_failed", Message: "Eingaben prüfen", Fields: map[string]string{id: fmt.Sprint(err)}}
}
