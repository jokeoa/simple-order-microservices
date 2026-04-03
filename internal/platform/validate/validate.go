package validate

import "github.com/go-playground/validator/v10"

type Validator struct {
	engine *validator.Validate
}

func New() *Validator {
	return &Validator{engine: validator.New(validator.WithRequiredStructEnabled())}
}

func (v *Validator) Struct(value any) error {
	return v.engine.Struct(value)
}
