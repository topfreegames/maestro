package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/topfreegames/maestro/internal/validations"
)

func parseValidationError(unparsedError error) error {
	parsedError := unparsedError
	switch unparsedErr := unparsedError.(type) {
	case validator.ValidationErrors:
		var errorMsg []string
		errorMsg = append(errorMsg, "Validation Error:")
		for _, err := range unparsedErr {
			translator := validations.GetDefaultTranslator()
			translatedMessage := fmt.Sprintf("%s: %s", err.Namespace(), err.Translate(translator))
			errorMsg = append(errorMsg, translatedMessage)
		}
		parsedError = errors.New(strings.Join(errorMsg, "\n"))
	}

	return parsedError
}
