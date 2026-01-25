package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

const runtimeContextKey = "oneagent_runtime"

func InjectRuntime(rt *runtime.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(runtimeContextKey, rt)
		c.Next()
	}
}

func GetRuntime(c *gin.Context) *runtime.Runtime {
	if c == nil {
		return nil
	}
	val, ok := c.Get(runtimeContextKey)
	if !ok {
		return nil
	}
	rt, _ := val.(*runtime.Runtime)
	return rt
}
