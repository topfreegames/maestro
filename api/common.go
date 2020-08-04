// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

// NOTE(lhahn): This function has race conditions. It is not safe to be called from multiple threads.
func getOperationManager(
	ctx context.Context,
	app *App,
	schedulerName, opName string,
	logger logrus.FieldLogger,
	mr *models.MixedMetricsReporter,
) (*models.OperationManager, error) {
	timeoutSec := app.Config.GetInt("updateTimeoutSeconds")

	opManager := models.NewOperationManager(schedulerName, app.RedisClient.Trace(ctx), logger)

	currOperation, err := opManager.CurrentOperation()
	if err != nil {
		return nil, err
	}
	if currOperation != "" {
		return nil, fmt.Errorf("operation key already in progress: %s", currOperation)
	}

	_ = mr.WithSegment(models.SegmentPipeExec, func() error {
		return opManager.Start(time.Duration(timeoutSec)*time.Second, opName)
	})
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
