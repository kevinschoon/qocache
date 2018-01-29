package query

import (
	"encoding/json"
	"fmt"
	qf "github.com/tobgu/qframe"
	"github.com/tobgu/qframe/filter"
	"strings"
)

// TODO: It is possible that most of the functionality here would actually fit better in the QFrame
//       Or even in an own, query, repository together with some of the query related functionality
//       in QFrame.

type query struct {
	Select   interface{} `json:"select,omitempty"`
	Where    interface{} `json:"where,omitempty"`
	OrderBy  []string    `json:"order_by,omitempty"`
	GroupBy  []string    `json:"group_by,omitempty"`
	Distinct []string    `json:"distinct,omitempty"`
	Offset   int         `json:"offset,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	From     *query      `json:"from,omitempty"`
}

func unMarshalFilterClauses(input []interface{}) ([]qf.Clause, error) {
	result := make([]qf.Clause, 0, len(input))
	for _, x := range input {
		c, err := unMarshalFilterClause(x)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}

	return result, nil
}

func unMarshalFilterClause(input interface{}) (qf.Clause, error) {
	var c qf.Clause = qf.Null()
	if input == nil {
		return c, c.Err()
	}

	clause, ok := input.([]interface{})
	if !ok {
		return c, fmt.Errorf("malformed filter clause, expected list of clauses, was: %v", input)
	}

	if len(clause) < 2 {
		return c, fmt.Errorf("malformed filter clause, too short: %v", clause)
	}

	operator, ok := clause[0].(string)
	if !ok {
		return c, fmt.Errorf("malformed filter clause, expected operator string, was: %v", clause[0])
	}

	switch operator {
	case "and", "AND":
		subClauses, err := unMarshalFilterClauses(clause[1:])
		if err != nil {
			return c, err
		}
		c = qf.And(subClauses...)
	case "or", "OR":
		subClauses, err := unMarshalFilterClauses(clause[1:])
		if err != nil {
			return c, err
		}
		c = qf.Or(subClauses...)
	case "not", "NOT":
		if len(clause) != 2 {
			return c, fmt.Errorf("invalid 'not' filter clause length, expected [not, [...]], was: %v", clause)
		}

		subClause, err := unMarshalFilterClause(clause[1])
		if err != nil {
			return c, err
		}
		c = qf.Not(subClause)
	default: // Comparisons: <, >, =, ...
		if len(clause) != 3 {
			return c, fmt.Errorf("invalid filter clause length, expected [operator, column, value], was: %v", clause)
		}

		colName, ok := clause[1].(string)
		if !ok {
			return c, fmt.Errorf("invalid column name, expected string, was: %v", clause[1])
		}

		c = qf.Filter(filter.Filter{Comparator: filter.Comparator(operator), Column: colName, Arg: clause[2]})
	}

	return c, c.Err()
}

func unMarshalSelectClause(input interface{}) (qf.Select, error) {
	if input == nil {
		return qf.Select(nil), nil
	}

	inputSlice, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("malformed select, must be a list, was: %v", inputSlice)
	}

	return qf.Select(inputSlice), nil
}

func unMarshalOrderByClause(input []string) []qf.Order {
	result := make([]qf.Order, len(input))
	for i, s := range input {
		if strings.HasPrefix(s, "-") {
			result[i] = qf.Order{Column: s[1:], Reverse: true}
		} else {
			result[i] = qf.Order{Column: s, Reverse: false}
		}
	}

	return result
}

func New(qString string) (query, error) {
	q := query{}
	err := json.Unmarshal([]byte(qString), &q)
	return q, err
}

func Query(f qf.QFrame, qString string) (qf.QFrame, error) {
	q, err := New(qString)
	if err != nil {
		return qf.QFrame{}, err
	}

	return q.Query(f)
}

func intMin(x, y int) int {
	if x < y {
		return x
	}

	return y
}

func (q query) slice(f qf.QFrame) qf.QFrame {
	stop := f.Len()
	if q.Limit > 0 {
		stop = intMin(stop, q.Offset+q.Limit)
	}

	return f.Slice(q.Offset, stop)
}

func (q query) Query(f qf.QFrame) (qf.QFrame, error) {
	filterClause, err := unMarshalFilterClause(q.Where)
	if err != nil {
		return f, err
	}

	selectClause, err := unMarshalSelectClause(q.Select)
	if err != nil {
		return f, err
	}

	newF := filterClause.Filter(f)
	newF = newF.Sort(unMarshalOrderByClause(q.OrderBy)...)
	newF = selectClause.Select(newF)

	// TODO: Add info about original frame length
	newF = q.slice(newF)
	return newF, newF.Err
}
