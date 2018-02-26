package api

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/topfreegames/maestro/models"
)

func getOperationManager(
	app *App,
	schedulerName, opName string,
	logger logrus.FieldLogger,
) *models.OperationManager {
	timeoutSec := app.Config.GetInt("updateTimeoutSeconds")

	operationManager := models.NewOperationManager(schedulerName, app.RedisClient, logger)
	operationManager.Start(time.Duration(timeoutSec)*time.Second, opName)

	logger.Infof("operation key: %s", operationManager.GetOperationKey())
	return operationManager
}
