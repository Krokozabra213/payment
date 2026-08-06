package apperror

import "log/slog"

type Field struct {
	Key   string
	Value any
}

type Fields []Field

func (f Fields) ToSlogArgs() []any {
	args := make([]any, 0, len(f)*2)
	for _, f := range f {
		args = append(args, f.Key, f.Value)
	}
	return args
}

func (f Fields) ToAttrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, len(f))
	for _, field := range f {
		attrs = append(attrs, slog.Any(field.Key, field.Value))
	}
	return attrs
}

func (f Fields) Add(fields ...Field) Fields {
	newFields := make(Fields, 0, len(f)+len(fields))
	newFields = append(newFields, f...)
	newFields = append(newFields, fields...)
	return newFields
}

func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}
