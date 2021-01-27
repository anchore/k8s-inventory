package errors

import "fmt"

type KaiError struct {
	Error error    `json:"error"`
	Msg   []string `json:"msg"`
}

func NewFromError(err error, msg string) *KaiError {
	return &KaiError{
		Error: err,
		Msg:   []string{msg},
	}
}

func (n *KaiError) ToError() error {
	return fmt.Errorf("%s: %w", n.Msg, n.Error)
}
