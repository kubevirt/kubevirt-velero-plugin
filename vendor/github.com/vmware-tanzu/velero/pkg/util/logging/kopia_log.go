/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"context"

	"github.com/kopia/kopia/repo/logging"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logrusCore adapts a logrus.FieldLogger to a zapcore.Core so that Kopia's
// zap-based Logger type can route output through Velero's logrus pipeline.
// Kopia error logs are demoted to warn level to avoid disrupting Velero's
// workflow for non-critical Kopia errors.
type logrusCore struct {
	logger logrus.FieldLogger
	module string
}

func (c *logrusCore) Enabled(zapcore.Level) bool { return true }

func (c *logrusCore) With(fields []zapcore.Field) zapcore.Core {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	fl := make(logrus.Fields, len(enc.Fields))
	for k, v := range enc.Fields {
		fl[k] = v
	}

	return &logrusCore{
		logger: c.logger.WithFields(fl),
		module: c.module,
	}
}

func (c *logrusCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(ent, c)
}

func (c *logrusCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	l := c.logger.WithField("logModule", "kopia/"+c.module)

	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	if len(enc.Fields) > 0 {
		fl := make(logrus.Fields, len(enc.Fields))
		for k, v := range enc.Fields {
			fl[k] = v
		}

		l = l.WithFields(fl)
	}

	switch ent.Level {
	case zapcore.DebugLevel:
		l.Debug(ent.Message)
	case zapcore.InfoLevel:
		l.Info(ent.Message)
	case zapcore.WarnLevel:
		l.Warn(ent.Message)
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		l.WithField("sublevel", "error").Warn(ent.Message)
	default:
		l.Info(ent.Message)
	}

	return nil
}

func (c *logrusCore) Sync() error { return nil }

// SetupKopiaLog sets the Kopia log handler to the specific context, Kopia modules
// call the logger in the context to write logs
func SetupKopiaLog(ctx context.Context, logger logrus.FieldLogger) context.Context {
	return logging.WithLogger(ctx, func(module string) logging.Logger {
		core := &logrusCore{logger: logger, module: module}
		return zap.New(core).Sugar().Named("kopia/" + module)
	})
}
