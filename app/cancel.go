package app

type IsCanceller interface {
	IsCancel() bool
}

var _ IsCanceller = &CancelError{}
var _ error = &CancelError{}

type CancelError struct {
	Reason string
}

func (e *CancelError) Error() string {
	if e.Reason == "" {
		return "deployment cancelled"
	} else {
		return e.Reason
	}
}

func (e *CancelError) IsCancel() bool {
	return true
}
