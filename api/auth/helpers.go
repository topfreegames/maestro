package auth

import (
	"context"
	"strings"
)

const emailKey = "emailKey"

func EmailFromContext(ctx context.Context) string {
	payload := ctx.Value(emailKey)
	if payload == nil {
		return ""
	}
	return payload.(string)
}

// NewContextWithEmail adds the email from oauth into context
func NewContextWithEmail(ctx context.Context, email string) context.Context {
	c := context.WithValue(ctx, emailKey, email)
	return c
}

func IsAuth(email string, auths []string) bool {
	if auths == nil {
		return false
	}

	for _, auth := range auths {
		if auth == email {
			return true
		}
	}

	return false
}

const basicPayloadString = "basicPayload"

// NewContextWithBasicAuthOK gets if basic auth was sent and is ok
func NewContextWithBasicAuthOK(ctx context.Context) context.Context {
	c := context.WithValue(ctx, basicPayloadString, true)
	return c
}

func IsBasicAuthOkFromContext(ctx context.Context) bool {
	payload := ctx.Value(basicPayloadString)
	if payload == nil {
		return false
	}

	return payload.(bool)
}

func VerifyEmailDomain(email string, emailDomains []string) bool {
	for _, domain := range emailDomains {
		if strings.HasSuffix(email, domain) {
			return true
		}
	}
	return false
}
