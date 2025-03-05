package response

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"strings"
)

type Response struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

const (
	statusOk    = "OK"
	statusError = "Error"
)

func OK() Response {
	return Response{
		Status: statusOk,
	}
}

func Error(msg string) Response {
	return Response{
		Status: statusError,
		Error:  msg,
	}
}

func ValidationError(errs validator.ValidationErrors) Response {
	var errMsg []string
	for _, err := range errs {
		switch err.ActualTag() {
		case "required":
			errMsg = append(errMsg, fmt.Sprintf("%s is required field", err.Field()))
		default:
			errMsg = append(errMsg, fmt.Sprintf("%s is not valid", err.Field()))
		}
	}
	return Response{
		Status: statusError,
		Error:  strings.Join(errMsg, ", "),
	}
}
