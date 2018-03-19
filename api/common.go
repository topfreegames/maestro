// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

func getOperationManager(
	app *App,
	schedulerName, opName string,
	logger logrus.FieldLogger,
) (*models.OperationManager, error) {
	timeoutSec := app.Config.GetInt("updateTimeoutSeconds")

	opManager := models.NewOperationManager(schedulerName, app.RedisClient, logger)

	currOperation, err := opManager.CurrentOperation()
	if err != nil {
		return nil, err
	}
	if currOperation != "" {
		return nil, fmt.Errorf("operation key already in progress: %s", currOperation)
	}

	opManager.Start(time.Duration(timeoutSec)*time.Second, opName)

	logger.Infof("operation key: %s", opManager.GetOperationKey())
	return opManager, nil
}

func returnIfOperationManagerExists(
	app *App,
	w http.ResponseWriter,
	err error,
) bool {
	if err == nil {
		return false
	}

	if strings.Contains(err.Error(), "operation key already in progress") {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"reason":  err.Error(),
		})

		return true
	}

	app.HandleError(w, http.StatusInternalServerError, "redis failed on get operation key", err)
	return true
}
