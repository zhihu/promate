package prometheus

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zhihu/promate/mateql"
)

func CovertMateQuery(query string, terminal bool) (string, error) {
	expr, err := mateql.Parse(query)
	if err != nil {
		return "", err
	}
	_, expr = covertExpr("", expr, terminal)
	return string(expr.AppendString(nil)), nil
}

func covertExpr(name string, expr mateql.Expr, terminal bool) (string, mateql.Expr) {
	switch e := expr.(type) {
	case *mateql.MetricExpr:
		var filters []mateql.LabelFilter
		for i, filter := range e.LabelFilters {
			if filter.Label == "__name__" && strings.Contains(filter.Value, ".") {
				name, filters = ConvertGraphiteTarget(filter.Value, terminal)
				if name == "" || filters == nil {
					continue
				}
				e.LabelFilters = append(e.LabelFilters, filters...)
				filter.Value = name
			}
			e.LabelFilters[i] = filter
		}
		return name, e
	case *mateql.RollupExpr:
		name, e.Expr = covertExpr(name, e.Expr, terminal)
		return name, e
	case *mateql.FuncExpr:
		for i, arg := range e.Args {
			name, e.Args[i] = covertExpr(name, arg, terminal)
		}
		return name, e
	case *mateql.AggrFuncExpr:
		for i, arg := range e.Args {
			name, e.Args[i] = covertExpr(name, arg, terminal)
		}
		for i, arg := range e.Modifier.Args {
			if len(arg) > 1 && arg[0] == 'g' && len(name) > 0 {
				if gi, err := strconv.Atoi(arg[1:]); err == nil {
					e.Modifier.Args[i] = fmt.Sprintf("__%s_g%d__", name, gi)
				}
			}
		}
		return name, e
	case *mateql.BinaryOpExpr:
		name, e.Left = covertExpr(name, e.Left, terminal)
		name, e.Right = covertExpr(name, e.Right, terminal)
		return name, e
	default:
		return name, e
	}
}
