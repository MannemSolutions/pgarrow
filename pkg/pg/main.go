package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

var (
	log      *zap.SugaredLogger
	quickLog *zap.Logger
	ctx      context.Context
	typeMap  *pgtype.Map
)

func InitLogger(logger *zap.SugaredLogger) {
	log = logger
	quickLog = logger.Desugar()
}
func InitContext(c context.Context) {
	ctx = c
}
