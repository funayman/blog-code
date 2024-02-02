package handler

import "errors"

// validationError is a lazy way for me to return "specific" errors and handle
// them accordingly in the handler func
type validationError string

// Error fulfills the error interface returning its value
func (ve validationError) Error() string {
	return string(ve)
}

// isValidationError does a quick check to determine error type
func isValidationError(err error) bool {
	var ve validationError
	return errors.As(err, &ve)
}
