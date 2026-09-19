package model

import "net/http"

// ErrorResponse is the canonical JSON error model for API responses.
type ErrorResponse struct {
	Code      int    `json:"code"`
	Error     string `json:"error"`
	Message   string `json:"message,omitempty"`
	Details   string `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorView is the view model for the HTML error page (error.html).
type ErrorView struct {
	Status    int
	Title     string
	Message   string
	Details   string
	RequestID string
	ShowLogin bool
}

// HTTPStatusText returns a user-friendly title for a status code.
func HTTPStatusText(code int) string {
	if t := http.StatusText(code); t != "" {
		return t
	}
	return "Unknown Error"
}

// NewErrorResponse builds an ErrorResponse.
func NewErrorResponse(code int, message, details, requestID string) ErrorResponse {
	return ErrorResponse{
		Code:      code,
		Error:     HTTPStatusText(code),
		Message:   message,
		Details:   details,
		RequestID: requestID,
	}
}

// AppError wraps an HTTP status with a user-facing message and internal detail.
type AppError struct {
	Status  int
	Message string
	Details string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Details != "" {
		return e.Details
	}
	return e.Message
}

// NewAppError creates an AppError.
func NewAppError(status int, message, details string) *AppError {
	return &AppError{Status: status, Message: message, Details: details}
}

func (e *AppError) WithErr(err error) *AppError {
	e.Err = err
	if e.Details == "" && err != nil {
		e.Details = err.Error()
	}
	return e
}
