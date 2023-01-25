package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

var (
	log             *zap.SugaredLogger
	quickLog        *zap.Logger
	ctx             context.Context
	typeMap         *pgtype.Map
	oidToPgType     map[uint32]string
	oidToPgCategory map[uint32]uint8
)

func InitLogger(logger *zap.SugaredLogger) {
	log = logger
	quickLog = logger.Desugar()
}
func InitContext(c context.Context) {
	ctx = c
}
