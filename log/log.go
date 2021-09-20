package log

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/rs/zerolog"
	"github.com/screenleap/zerologr"
)

func New() logr.Logger {
	zl := zerolog.New(os.Stderr)
	return zerologr.New(&zl)
}
