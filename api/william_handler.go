// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/extensions/middleware"
	"net/http"
)

type WilliamHandler struct {
	App *App
}

func NewWilliamHandler(a *App) *WilliamHandler {
	return &WilliamHandler{App: a}
}

func (h *WilliamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")

	logger := middleware.GetLogger(ctx).WithFields(logrus.Fields{
		"source":    "williamHandler",
		"operation": "am",
		"prefix":    prefix,
	})

	logger.Debug("getting permissions for prefix")
	permissions, err := h.App.William.Permissions(h.App.DBClient.WithContext(ctx), prefix)
	if err != nil {
		logger.WithError(err).Error("error building permissions")
		h.App.HandleError(w, http.StatusInternalServerError, "internal server error", err)
		return
	}

	logger.Debugf("permissions found: %d", len(permissions))

	bs, err := json.Marshal(permissions)
	if err != nil {
		logger.WithError(err).Error("error encoding json response")
		h.App.HandleError(w, http.StatusInternalServerError, "internal server error", err)
		return
	}

	WriteBytes(w, http.StatusOK, bs)
}
