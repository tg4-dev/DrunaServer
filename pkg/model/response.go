package model

type ErrorBody struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
	// Field names the request field the error is about, e.g. "username"
	// on a 409 from sign-up. Omitted when the error is not about one field.
	Field string `json:"field,omitempty"`
}

type APIResponse struct {
	Data  interface{} `json:"data"`
	Error *ErrorBody  `json:"error"`
}
