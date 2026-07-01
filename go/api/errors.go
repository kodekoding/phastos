package api

import (
	"net/http"
)

type HttpError struct {
	Message    string      `json:"message"`
	Code       string      `json:"code"`
	Status     int         `json:"-"`
	TraceId    string      `json:"trace_id"`
	Data       interface{} `json:"data,omitempty"`
	CallerPath string      `json:"caller_path,omitempty"`
}

type ErrorOption func(*HttpError)

func (e *HttpError) Write(w http.ResponseWriter) {
	w.WriteHeader(e.Status)
	WriteJson(w, e)
}

func (e *HttpError) Error() string {
	return e.Message
}

func WithErrorCode(code string) ErrorOption {
	return func(e *HttpError) {
		e.Code = code
	}
}

func WithErrorStatus(status int) ErrorOption {
	return func(e *HttpError) {
		e.Status = status
	}
}

func WithErrorMessage(message string) ErrorOption {
	return func(e *HttpError) {
		e.Message = message
	}
}

func WithErrorData(data interface{}) ErrorOption {
	return func(e *HttpError) {
		e.Data = data
	}
}

func WithTraceId(traceId string) ErrorOption {
	return func(e *HttpError) {
		e.TraceId = traceId
	}
}

func WithErrorCallerPath(callerPath string) ErrorOption {
	return func(e *HttpError) {
		e.CallerPath = callerPath
	}
}

func NewErr(opts ...ErrorOption) *HttpError {
	httpErr := &HttpError{
		Status:  http.StatusInternalServerError,
		Code:    "SERVER_ERROR",
		Message: "an error occured",
	}

	for _, opt := range opts {
		opt(httpErr)
	}

	return httpErr
}

func InternalServerError(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  500,
	}
}

func UnprocessableEntity(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  422,
	}
}

func NotFound(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  404,
	}
}

func MethodNotAllowed(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  405,
	}
}

func Unauthorized(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  401,
	}
}

func Forbidden(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  403,
	}
}

func BadRequest(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  400,
	}
}

func TooManyRequest(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  429,
	}
}

type constraintRegistryEntry struct {
	Code    string
	Message string
}

// constraintRegistry maps PostgreSQL constraint names to specific error codes and messages.
// When a constraint violation is detected, SetError looks up the constraint name here
// before falling back to generic DATA_CONFLICT / CONSTRAINT_VIOLATION.
var constraintRegistry = map[string]constraintRegistryEntry{
	"users_email_key":                                             {Code: "EMAIL_ALREADY_EXISTS", Message: "Email already registered"},
	"users_ktp_no_key":                                            {Code: "NIK_ALREADY_EXISTS", Message: "NIK already exists"},
	"clients_code_key":                                            {Code: "CLIENT_CODE_EXISTS", Message: "Client code already exists"},
	"clients_email_key":                                           {Code: "CLIENT_EMAIL_EXISTS", Message: "Client email already exists"},
	"accounts_alias_name_key":                                     {Code: "ALIAS_NAME_EXISTS", Message: "Alias name already exists"},
	"client_locations_code_key":                                   {Code: "LOCATION_CODE_EXISTS", Message: "Location code already exists"},
	"payroll_codes_code_key":                                      {Code: "PAYROLL_CODE_EXISTS", Message: "Payroll code already exists"},
	"user_group_privillege_user_group_id_menu_id_key":             {Code: "DUPLICATE_GROUP_PRIVILEGE", Message: "Duplicate group privilege"},
	"unique_employee_id_string":                                   {Code: "EMPLOYEE_ID_EXISTS", Message: "Employee ID already exists"},
	"idx_job_levels_name_unique":                                  {Code: "JOB_LEVEL_NAME_EXISTS", Message: "Job level name already exists"},
	"unique_account_position_level_combination":                   {Code: "POSITION_LEVEL_COMBINATION_EXISTS", Message: "Position-level combination already exists"},
	"unique_shift_name":                                           {Code: "SHIFT_NAME_EXISTS", Message: "Shift name already exists"},
	"employee_period_payslips":                                    {Code: "PAYSLIP_PERIOD_EXISTS", Message: "Payslip for this period already exists"},
	"unique_employee_and_date_idx":                                {Code: "PAYSLIP_PERIOD_EXISTS", Message: "Payslip for this period already exists"},
}

func ConflictError(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  409,
	}
}
