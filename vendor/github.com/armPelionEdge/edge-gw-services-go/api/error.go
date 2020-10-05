package api

import (
	"errors"
	"fmt"
)

type APIError struct {
	Status int
}

func (apiError APIError) Error() string {
	return fmt.Sprintf("Received response %d", apiError.Status)
}

var ENoAccountId = errors.New("No account Id provided")
