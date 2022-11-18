package pg

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

var (
	log     *zap.SugaredLogger
	ctx     context.Context
	typeMap *pgtype.Map
)

func InitLogger(logger *zap.SugaredLogger) {
	log = logger
}
func InitContext(c context.Context) {
	ctx = c
}
