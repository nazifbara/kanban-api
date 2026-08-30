package users

import (
	"errors"
)

func ValidateParams(params UserParam) error {
	errs := []error{}

	if params.Email == "" {
		errs = append(errs, errors.New("body.email is required"))
	}
	if params.LastName == "" {
		errs = append(errs, errors.New("body.last_name is required"))
	}
	if params.FirstName == "" {
		errs = append(errs, errors.New("body.first_name is required"))
	}
	if params.Password == "" {
		errs = append(errs, errors.New("body.password is required"))
	}
	if len(params.Password) < 8 {
		errs = append(errs, errors.New("body.password must be at least 8 characters long"))
	}

	return errors.Join(errs...)
}
