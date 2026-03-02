package exitcode

// Exit codes for structured error reporting.
const (
	Success          = 0
	General          = 1
	Network          = 2
	Auth             = 3
	InsufficientFunds = 4
	MessageTooLarge  = 5
)

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// Wrap creates an ExitError with the given code and underlying error.
func Wrap(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}
