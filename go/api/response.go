package api

import "github.com/kodekoding/phastos/v2/go/database"

type Response struct {
	Message  string                     `json:"message,omitempty"`
	Data     interface{}                `json:"data,omitempty"`
	Err      error                      `json:"error,omitempty"`
	MetaData *database.ResponseMetaData `json:"metadata,omitempty"`
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
	if selectResponseData, valid := data.(*database.SelectResponse); valid {
		if selectResponseData.ResponseMetaData != nil {
			resp.MetaData = selectResponseData.ResponseMetaData
		}
		resp.Data = selectResponseData.Data
	}
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
