package rcommons

import "github.com/google/uuid"

func calculateQueueName(appName string, suffix string, randSufix bool) string {
	if suffix == "" {
		suffix = "default.queue"
	}
	if randSufix {
		return appName + "." + suffix + "." + uuid.New().String()
	}
	return appName + "." + suffix
}
