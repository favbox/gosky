package recovery

import (
	"context"
	"fmt"
	"testing"

	"github.com/favbox/gosky/wind/pkg/app"
	"github.com/favbox/gosky/wind/pkg/common/hlog"
	"github.com/favbox/gosky/wind/pkg/common/utils"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestDefaultOption(t *testing.T) {
	opts := newOptions()
	assert.Equal(t, fmt.Sprintf("%p", defaultRecoveryHandler), fmt.Sprintf("%p", opts.recoveryHandler))
}

func TestOption(t *testing.T) {
	opts := newOptions(WithRecoveryHandler(myRecoveryHandler))
	assert.Equal(t, fmt.Sprintf("%p", myRecoveryHandler), fmt.Sprintf("%p", opts.recoveryHandler))
}

func myRecoveryHandler(c context.Context, ctx *app.RequestContext, err any, stack []byte) {
	hlog.SystemLogger().CtxErrorf(c, "[恐慌恢复] 恐慌已恢复:\n%s\n%s\n", err, stack)
	ctx.JSON(consts.StatusNotImplemented, utils.H{"msg": err.(string)})
}
