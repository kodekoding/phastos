package slack

type Reminder struct {
	Id         string `json:"id"`
	Creator    string `json:"creator"`
	User       string `json:"user"`
	Text       string `json:"text"`
	Recurring  bool   `json:"recurring"`
	Time       int    `json:"time"`
	CompleteTs int    `json:"complete_ts"`
}
