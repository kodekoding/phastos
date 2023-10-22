package cron

type (
	Response struct {
		err         error
		processName string
	}
)

func NewResponse() *Response {
	return &Response{}
}

func (r *Response) SetProcessName(name string) *Response {
	r.processName = name
	return r
}

func (r *Response) SetError(err error) *Response {
	r.err = err
	return r
}
