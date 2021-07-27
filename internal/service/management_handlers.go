package service

import "net/http"

type Handlers = map[string]http.Handler

func ProvideHandlers() Handlers {
	return Handlers{}
}
