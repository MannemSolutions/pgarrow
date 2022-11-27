package internal

import (
	"context"

	"github.com/mannemsolutions/pgarrrow/pkg/kafka"
	"github.com/mannemsolutions/pgarrrow/pkg/pg"
	"github.com/mannemsolutions/pgarrrow/pkg/rabbitmq"
)

var (
	arrowCtx           context.Context
	arrowCtxCancelFunc context.CancelFunc
)

func initContext() {
	arrowCtx, arrowCtxCancelFunc = context.WithCancel(context.Background())
	kafka.InitContext(arrowCtx)
	pg.InitContext(arrowCtx)
	rabbitmq.InitContext(arrowCtx)
}
