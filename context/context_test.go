package context_test

import (
	"context"
	"sync"
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

func Test_AddFieldsToContext(t *testing.T) {
	t.Run("store and retrieve fields", func(t *testing.T) {
		ctx := context.Background()
		fields := []map[string]interface{}{
			{"request_id": "abc-123"},
			{"method": "GET"},
		}
		ctx = goctx.AddFieldsToContext(ctx, fields)

		got := goctx.GetFieldsFromContext(ctx)
		assert.Equal(t, fields, got)
	})

	t.Run("returns empty slice when no fields set", func(t *testing.T) {
		ctx := context.Background()
		got := goctx.GetFieldsFromContext(ctx)
		assert.Empty(t, got)
		assert.NotNil(t, got)
	})
}

func Test_MutableFields(t *testing.T) {
	t.Run("add and get fields", func(t *testing.T) {
		mf := goctx.NewMutableFields()
		mf.AddField(map[string]interface{}{"key1": "val1"})
		mf.AddField(map[string]interface{}{"key2": 42})

		fields := mf.GetFields()
		assert.Len(t, fields, 2)
		assert.Equal(t, "val1", fields[0]["key1"])
		assert.Equal(t, 42, fields[1]["key2"])
	})

	t.Run("new MutableFields has no fields", func(t *testing.T) {
		mf := goctx.NewMutableFields()
		assert.Empty(t, mf.GetFields())
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		mf := goctx.NewMutableFields()
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				mf.AddField(map[string]interface{}{"n": n})
			}(i)
		}

		wg.Wait()
		assert.Len(t, mf.GetFields(), 100)
	})
}

func Test_ContextKeyString(t *testing.T) {
	// ContextKeyLoggerFields is exported; verify its String() representation.
	assert.Equal(t, "loggerFields", goctx.ContextKeyLoggerFields.String())
}
