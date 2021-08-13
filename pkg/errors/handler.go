package errors

import (
	"github.com/triton-io/triton/pkg/log"
)

func HandleError(err error, message string) {
	log.WithError(err).Error(message)
}
