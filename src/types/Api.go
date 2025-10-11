package types

type ApiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   error  `json:"error,omitempty"`
}
