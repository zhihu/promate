package prometheus

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/zhihu/promate/mateql"
)

var builderPool = &sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 1024))
	},
}

type LabelFilters []mateql.LabelFilter

func (l LabelFilters) Build(name string) (selector string) {
	builder := builderPool.Get().(*bytes.Buffer)
	defer builderPool.Put(builder)
	builder.Reset()

	builder.WriteString(`{__name__="`)
	builder.WriteString(name)
	builder.WriteByte('"')

	for _, filter := range l {
		if filter.IsRegexp {
			builder.WriteByte(',')
			builder.WriteString(filter.Label)
			builder.WriteString(`=~"`)
			builder.WriteString(filter.Value)
			builder.WriteByte('"')
		} else if filter.IsNegative {
			builder.WriteByte(',')
			builder.WriteString(filter.Label)
			builder.WriteString(`!="`)
			builder.WriteString(filter.Value)
			builder.WriteByte('"')
		} else {
			builder.WriteByte(',')
			builder.WriteString(filter.Label)
			builder.WriteString(`="`)
			builder.WriteString(filter.Value)
			builder.WriteByte('"')
		}
	}

	builder.WriteString(`}`)
	return builder.String()
}

func ConvertGraphiteTarget(query string, terminal bool) (string, LabelFilters) {
	nodes := strings.Split(query, ".")
	length := len(nodes)
	name := strings.ReplaceAll(nodes[0], "-", "_")

	filters := make(LabelFilters, 0, length)
	for i := 1; i < length; i++ {
		node := nodes[i]
		if node == "*" {
			continue
		}
		value, isRegex, err := globToRegexPattern(node)
		if err != nil {
			return "", nil
		}

		filters = append(filters, mateql.LabelFilter{
			Label:    labelName(name, i),
			Value:    value,
			IsRegexp: isRegex,
		})
	}
	if terminal {
		filters = append(filters, mateql.LabelFilter{
			Label: labelName(name, length),
			Value: "",
		})
	}

	return name, filters
}

func ConvertQueryLabel(query string) (prefix, label string, fast bool) {
	builder := builderPool.Get().(*bytes.Buffer)
	defer builderPool.Put(builder)
	builder.Reset()

	nodes := strings.Split(query, ".")
	length := len(nodes)
	name := strings.ReplaceAll(nodes[0], "-", "_")

	builder.WriteString(name)
	for i := 1; i < length-1; i++ {
		builder.WriteByte('.')
		builder.WriteString(nodes[i])
	}
	builder.WriteByte('.')

	return builder.String(), labelName(name, length-1), length == 2
}

func ConvertPrometheusMetric(name string, metric map[string]string) string {
	// Detect error response https://github.com/VictoriaMetrics/VictoriaMetrics/issues/360
	__name__, ok := metric["__name__"]
	if ok && __name__ != name {
		return ""
	}

	builder := builderPool.Get().(*bytes.Buffer)
	defer builderPool.Put(builder)
	builder.Reset()

	builder.WriteString(name)
	for i := 1; i < len(metric)+1; i++ {
		if value, ok := metric[fmt.Sprintf("__%s_g%d__", name, i)]; ok {
			builder.WriteByte('.')
			builder.WriteString(value)
		}
	}
	return builder.String()
}

func labelName(name string, i int) string {
	return fmt.Sprintf("__%s_g%d__", name, i)
}
