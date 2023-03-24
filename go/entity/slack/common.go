package slack

type ResponseType interface {
	Channel | Reminder | OauthAccess
}

type Response[T ResponseType] struct {
	Ok    bool `json:"ok"`
	Data  T
	Error string `json:"error,omitempty"`
}
