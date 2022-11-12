package helpers

type UserResponse struct {
	Error bool        `json:"error"`
	Data  interface{} `json:"data"`
}
