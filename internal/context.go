package internal

import (
	"context"
	"github.com/mannemsolutions/pgarrrow/pkg/kafka"
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
)

var (
	arrowCtx           context.Context
	arrowCtxCancelFunc context.CancelFunc
)

func initContext() {
	arrowCtx, arrowCtxCancelFunc = context.WithCancel(context.Background())
	kafka.InitContext(arrowCtx)
	pg.InitContext(arrowCtx)
}
