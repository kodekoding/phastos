package error

type RequestError struct {
	code int
	data map[string]interface{}
	err  error
}

func New(err error) *RequestError {
	return &RequestError{data: make(map[string]interface{}), err: err}
}

func (re *RequestError) AppendData(key string, data interface{}) *RequestError {
	re.data[key] = data
	return re
}

func (re *RequestError) GetData() map[string]interface{} {
	return re.data
}

func (re *RequestError) Error() string {
	return re.err.Error()
}

func (re *RequestError) SetCode(code int) *RequestError {
	re.code = code
	return re
}

func (re *RequestError) GetCode() int {
	return re.code
}
