package internal

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	go func() {
		for range c {
			arrowCtxCancelFunc()
		}
	}()
	kafka.InitContext(arrowCtx)
	pg.InitContext(arrowCtx)
	rabbitmq.InitContext(arrowCtx)
}
