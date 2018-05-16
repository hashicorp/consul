package msgfmt

import (
	"fmt"
	"github.com/v2pro/plz/msgfmt/jsonfmt"
	"reflect"
	"strings"
	"github.com/v2pro/plz/reflect2"
)


type Formatter interface {
	Format(space []byte, kv []interface{}) []byte
}

type Formatters []Formatter

func (formatters Formatters) Format(space []byte, kv []interface{}) []byte {
	for _, formatter := range formatters {
		space = formatter.Format(space, kv)
	}
	return space
}

type formatCompiler struct {
	sample     []interface{}
	format     string
	start      int
	levels     int
	varExpr    varExpr
	onByte     func(int, byte)
	formatters []Formatter
}

func compile(format string, sample []interface{}) Formatter {
	compiler := &formatCompiler{
		sample: sample,
		format: format,
	}
	compiler.onByte = compiler.normal
	compiler.compile()
	return Formatters(compiler.formatters)
}

func (compiler *formatCompiler) compile() {
	format := compiler.format
	for i := 0; i < len(format); i++ {
		compiler.onByte(i, format[i])
	}
	if reflect.ValueOf(compiler.onByte).Pointer() == reflect.ValueOf(compiler.endState).Pointer() {
		return
	}
	if reflect.ValueOf(compiler.onByte).Pointer() == reflect.ValueOf(compiler.normal).Pointer() {
		compiler.formatters = append(compiler.formatters,
			fixedFormatter(compiler.format[compiler.start:len(format)]))
	} else {
		compiler.invalidFormat(len(format)-1, "verb not properly ended")
	}
}

func (compiler *formatCompiler) normal(i int, b byte) {
	format := compiler.format
	if format[i] == '{' {
		compiler.formatters = append(compiler.formatters,
			fixedFormatter(format[compiler.start:i]))
		compiler.start = i + 1
		compiler.onByte = compiler.afterLeftCurlyBrace
	}
}

func (compiler *formatCompiler) afterLeftCurlyBrace(i int, b byte) {
	switch b {
	case '}':
		key := compiler.format[compiler.start:i]
		compiler.varExpr.key = key
		idx := compiler.findVarExprKey()
		if idx == -1 {
			compiler.invalidFormat(i, compiler.varExpr.key+" not found in args")
			return
		}
		sampleValue := compiler.sample[idx]
		switch sampleValue.(type) {
		case string:
			compiler.formatters = append(compiler.formatters, strFormatter(idx))
		case []byte:
			compiler.formatters = append(compiler.formatters, bytesFormatter(idx))
		default:
			compiler.formatters = append(compiler.formatters, &jsonFormatter{
				idx:     idx,
				encoder: jsonfmt.EncoderOf(reflect2.TypeOf(sampleValue)),
			})
		}
		compiler.start = i + 1
		compiler.onByte = compiler.normal
	case ',':
		key := compiler.format[compiler.start:i]
		compiler.varExpr.key = key
		compiler.start = i + 1
		compiler.onByte = compiler.afterKey
	default:
		// nothing
	}
}

func (compiler *formatCompiler) afterKey(i int, b byte) {
	switch b {
	case ',':
		compiler.varExpr.formatName = strings.TrimSpace(compiler.format[compiler.start:i])
		compiler.start = i + 1
		compiler.onByte = compiler.afterFormatterName
	case '}':
		compiler.varExpr.formatName = strings.TrimSpace(compiler.format[compiler.start:i])
		compiler.start = i + 1
		formatter, err := compiler.varExpr.newFormatter(compiler.sample)
		if err != nil {
			compiler.invalidFormat(i, err.Error())
			return
		}
		compiler.formatters = append(compiler.formatters, formatter)
		compiler.onByte = compiler.normal
	default:
		// nothing
	}
}

func (compiler *formatCompiler) afterFormatterName(i int, b byte) {
	switch b {
	case ',':
		argName := strings.TrimSpace(compiler.format[compiler.start:i])
		compiler.start = i + 1
		compiler.varExpr.formatArgs = append(compiler.varExpr.formatArgs, argName)
	case '}':
		argName := strings.TrimSpace(compiler.format[compiler.start:i])
		compiler.varExpr.formatArgs = append(compiler.varExpr.formatArgs, argName)
		compiler.start = i + 1
		formatter, err := compiler.varExpr.newFormatter(compiler.sample)
		if err != nil {
			compiler.invalidFormat(i, err.Error())
			return
		}
		compiler.formatters = append(compiler.formatters, formatter)
		compiler.onByte = compiler.normal
	default:
		// nothing
	}
}

func (compiler *formatCompiler) findVarExprKey() int {
	for i := 0; i < len(compiler.sample); i += 2 {
		key := compiler.sample[i].(string)
		if key == compiler.varExpr.key {
			return i + 1
		}
	}
	return -1
}

func (compiler *formatCompiler) invalidFormat(i int, err string) {
	compiler.onByte = compiler.endState
	compiler.formatters = []Formatter{invalidFormatter(fmt.Sprintf(
		"%s at %d %s", err, i, compiler.format))}
}

func (compiler *formatCompiler) endState(i int, b byte) {
}
