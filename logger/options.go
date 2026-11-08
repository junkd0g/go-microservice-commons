package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

// Options configures service logging without embedding application policies.
type Options struct {
	Service   string
	QuietInfo func(string) bool
	Transform func(zapcore.Entry, []zapcore.Field) (zapcore.Entry, []zapcore.Field)
}

// NewServiceLogger emits JSON to stdout with stable timestamps and unique fields.
func NewServiceLogger(options Options) (*Logger, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "ts"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(cfg), zapcore.Lock(os.Stdout), zap.InfoLevel)
	return NewWithCore(core, options), nil
}

// NewWithCore applies service policies to a supplied core, including test sinks.
func NewWithCore(core zapcore.Core, options Options) *Logger {
	if options.Transform != nil {
		core = transformCore{Core: core, transform: options.Transform}
	}
	var fields []zapcore.Field
	if options.Service != "" {
		fields = append(fields, zap.String("service", options.Service))
	}
	core = quietCore{Core: uniqueCore{Core: core, fields: fields}, quiet: options.QuietInfo}
	return &Logger{logger: zap.New(core)}
}

type transformCore struct {
	zapcore.Core
	transform func(zapcore.Entry, []zapcore.Field) (zapcore.Entry, []zapcore.Field)
}

func (c transformCore) With(fields []zapcore.Field) zapcore.Core {
	return transformCore{c.Core.With(fields), c.transform}
}
func (c transformCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry, fields = c.transform(entry, fields)
	return c.Core.Write(entry, fields)
}

type quietCore struct {
	zapcore.Core
	quiet func(string) bool
}

func (c quietCore) With(f []zapcore.Field) zapcore.Core { return quietCore{c.Core.With(f), c.quiet} }
func (c quietCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(e.Level) {
		return ce
	}
	if e.Level == zapcore.InfoLevel && c.quiet != nil && c.quiet(e.Message) {
		return ce
	}
	return ce.AddCore(e, c)
}

// uniqueCore defers context encoding until Write so each key is emitted once.
// Copies on With keep concurrent request contexts isolated; the last value wins.
type uniqueCore struct {
	zapcore.Core
	fields []zapcore.Field
}

func (c uniqueCore) With(fields []zapcore.Field) zapcore.Core {
	copyFields := append([]zapcore.Field(nil), c.fields...)
	return uniqueCore{Core: c.Core, fields: append(copyFields, fields...)}
}
func (c uniqueCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	all := append(append([]zapcore.Field(nil), c.fields...), fields...)
	out := make([]zapcore.Field, 0, len(all))
	positions := make(map[string]int, len(all))
	for _, field := range all {
		if at, exists := positions[field.Key]; exists {
			out[at] = field
		} else {
			positions[field.Key] = len(out)
			out = append(out, field)
		}
	}
	return c.Core.Write(entry, out)
}
