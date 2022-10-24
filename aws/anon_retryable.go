package nsaws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

var _ retry.IsErrorRetryable = AnonRetryable(func(err error) aws.Ternary {
	return aws.FalseTernary
})

type AnonRetryable func(error) aws.Ternary

func (a AnonRetryable) IsErrorRetryable(err error) aws.Ternary {
	return a(err)
}
