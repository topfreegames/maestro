package handlers

import (
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/topfreegames/maestro/internal/validations"
	"strings"
)

func parseValidationError(unparsedError error) error {
	parsedError := unparsedError
	switch unparsedError.(type) {
	case validator.ValidationErrors:
		var errorMsg []string
		errorMsg = append(errorMsg, "Validation Error:")
		for _, error := range unparsedError.(validator.ValidationErrors) {
			translator := validations.GetDefaultTranslator()
			translatedMessage := fmt.Sprintf("%s: %s", error.Namespace(), error.Translate(translator))
			errorMsg = append(errorMsg, translatedMessage)
		}
		parsedError = errors.New(strings.Join(errorMsg, "\n"))
	}

	return parsedError
}


