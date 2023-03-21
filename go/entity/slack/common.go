package slack

type ResponseType interface {
	Channel | Reminder
}

type Response[T ResponseType] struct {
	Ok    bool   `json:"ok"`
	Data  T      `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
