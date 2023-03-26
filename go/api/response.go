package api

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Err     error       `json:"error"`
}

func NewResponse() *Response {
	return &Response{}
}

func (resp *Response) SetMessage(msg string) *Response {
	resp.Message = msg
	return resp
}

func (resp *Response) SetData(data interface{}) *Response {
	resp.Data = data
	return resp
}

func (resp *Response) SetError(err error) *Response {
	resp.Err = err
	return resp
}

func (resp *Response) SetHTTPError(err *HttpError) *Response {
	resp.Err = err
	return resp
}
