package models

import (
	"encoding/json"
	"net/http"
)

// WriterWrapper wrapps and implements a http.ResponseWriter
// to save other variables
type WriterWrapper struct {
	writer http.ResponseWriter
	status int
	msg    []byte
}

//NewWriterWrapper returns a new WriterWrapper
func NewWriterWrapper(writer http.ResponseWriter) *WriterWrapper {
	return &WriterWrapper{
		writer: writer,
	}
}

// Header just executes http.ResponseWriter method
func (w *WriterWrapper) Header() http.Header {
	return w.writer.Header()
}

// Write executes http.ResponseWriter method and saves the message
func (w *WriterWrapper) Write(b []byte) (int, error) {
	w.msg = b
	return w.writer.Write(b)
}

// WriteHeader executes http.ResponseWriter method and saves status
func (w *WriterWrapper) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}

	w.writer.WriteHeader(status)
}

//Status returns saved status
func (w *WriterWrapper) Status() string {
	if w.status >= 500 {
		return "500"
	} else if w.status >= 400 {
		return "400"
	}

	return "200"
}

// Message returns the message sent as response on json format
func (w *WriterWrapper) Message() map[string]interface{} {
	var res map[string]interface{}
	err := json.Unmarshal(w.msg, &res)
	if err != nil {
		return nil
	}

	return res
}
