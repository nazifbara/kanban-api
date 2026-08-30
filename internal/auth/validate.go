package auth

import (
	"errors"

	"github.com/nazifbara/kanban-api/internal/utils"
)

func ValidateParams(params Params) error {
	errs := []error{}
	if params.Email == "" {
		errs = append(errs, errors.New("body.email is required"))
	}
	if !utils.IsValidEmail(params.Email) {
		errs = append(errs, errors.New("body.email is invalid"))
	}
	if params.Password == "" {
		errs = append(errs, errors.New("body.password is required"))
	}

	return errors.Join(errs...)
}
