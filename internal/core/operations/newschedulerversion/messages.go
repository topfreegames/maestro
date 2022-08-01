package newschedulerversion

const (
	validationTimeoutMessage = `The GRU could not be validated. Maestro got timeout waiting for the GRU with ID: %s to be ready. You can check if
		the GRU image is stable on the its logs. If you could not spot any issues, contact the Maestro's responsible team for helping.`

	validationPodInErrorMessage = `The GRU could not be validated. The room created for validation with ID %s is entering in error state. You can check if
		the GRU image is stable on the its logs using the provided room id. Last event in the game room: %s.`

	validationUnexpectedErrorMessage = `The GRU could not be validated. Unexpected Error: %s - Contact the Maestro's responsible team for helping.`
)
