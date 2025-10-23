package context_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	goctx "github.com/junkd0g/go-microservice-commons/context"
	"github.com/junkd0g/go-microservice-commons/logger"
)

func Test_Logger(t *testing.T) {
	t.Run("Add successfully a logger and retrieve it", func(t *testing.T) {
		ctx := context.Background()
		log, _ := logger.NewLogger()
		ctx = goctx.AddLoggerToContex(ctx, log)
		assert.NotNil(t, ctx)
		loggerToTest, err := goctx.GetLoggerFromContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, loggerToTest)
	})

	t.Run("Add no logger found", func(t *testing.T) {
		ctx := context.Background()
		loggerToTest, err := goctx.GetLoggerFromContext(ctx)

		assert.Error(t, err)
		assert.Nil(t, loggerToTest)
	})
}
