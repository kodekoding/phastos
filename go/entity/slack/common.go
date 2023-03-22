package slack

type ResponseType interface {
	Channel | Reminder | OauthAccess
}

type Response[T ResponseType] struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	T
}
