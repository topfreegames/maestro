// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package newversion

const (
	startingValidationMessageTemplate = "Major version detected, starting game room validation process..."

	enqueuedSwitchVersionMessageTemplate = "enqueued switch active version operation with id: %s"

	validationSuccessMessageTemplate = "%dº Attempt: Game room validation success!"

	allAttemptsFailedMessageTemplate = "All validation attempts have failed, operation aborted!"

	validationTimeoutMessageTemplate = `%dº Attempt: Got timeout waiting for the GRU with ID: %s to be ready. You can check if
		the GRU image is stable on its logs.`

	validationPodInErrorMessageTemplate = `%dº Attempt: The room created for validation with ID %s is entering in error state. You can check if
		the GRU image is stable on its logs using the provided room id. Last event in the game room: %q`

	validationUnexpectedErrorMessageTemplate = `%dº Attempt: Unexpected Error, contact the Maestro's responsible team for helping: %q`
)
