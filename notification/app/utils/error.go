package utils

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ErrObj struct {
	Error string
}

func NewErrObj(Error string) ErrObj {
	return ErrObj{Error: Error}
}

// handle error
func ErrorRes(err error) gin.H {
	var validationErrors []ErrObj

	if vErrors, ok := err.(validator.ValidationErrors); ok {
		for _, ve := range vErrors {
			validationErrors = append(validationErrors, NewErrObj(ve.Field()+": value cannot be empty!"))
		}
	} else {
		validationErrors = append(validationErrors, NewErrObj(err.Error()))
	}

	return gin.H{
		"status":  "error",
		"message": validationErrors,
	}
}
