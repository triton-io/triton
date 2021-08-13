package errors

import (
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const LastDeployInProgress = "last deploy in progress"
const TimeIntervalNotReached = "time interval not reached"
const InstanceNotUp = "instance is not up yet"

type RequeueError interface {
	RequeueAfter() time.Duration
}

type requeueAfterError struct {
	msg string
	err error

	requeueAfter time.Duration
}

var _ error = &requeueAfterError{}
var _ RequeueError = &requeueAfterError{}

func (e requeueAfterError) Error() string {
	return e.msg
}

func (e requeueAfterError) Is(target error) bool {
	return target.Error() == e.Error()
}

func (e requeueAfterError) Unwrap() error {
	return e.err
}

func (e *requeueAfterError) RequeueAfter() time.Duration {
	return e.requeueAfter
}

type HttpError struct {
	code int32
	msg  string
	err  error
}

var _ error = &HttpError{}

func (e HttpError) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.err)
}

func NewNotFound(msg string) *HttpError {
	return &HttpError{
		code: http.StatusNotFound,
		msg:  msg,
	}
}

func NewConflict(msg string, err error) *HttpError {
	return &HttpError{
		code: http.StatusConflict,
		msg:  msg,
		err:  err,
	}
}

// CodeForError returns the HTTP status for a particular error.
func CodeForError(err error) int32 {
	switch e := err.(type) {
	case *HttpError:
		return e.code
	case *apierrors.StatusError:
		return e.ErrStatus.Code
	default:
		return http.StatusInternalServerError
	}
}

func IsNotFound(err error) bool {
	return CodeForError(err) == http.StatusNotFound
}

func IsConflict(err error) bool {
	return CodeForError(err) == http.StatusConflict
}

func NewLastDeployInProgressError(requeueAfter time.Duration) error {
	return &requeueAfterError{msg: LastDeployInProgress, requeueAfter: requeueAfter}
}

func NewTimeIntervalNotReachedError(requeueAfter time.Duration) error {
	return &requeueAfterError{msg: TimeIntervalNotReached, requeueAfter: requeueAfter}
}

func NewInstanceNotUpError(requeueAfter time.Duration) error {
	return &requeueAfterError{msg: InstanceNotUp, requeueAfter: requeueAfter}
}
