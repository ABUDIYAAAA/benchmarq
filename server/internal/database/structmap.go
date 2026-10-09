package database

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
)

// Struct mapping for wide tables.
//
// Fields tagged `db:"column"` map to a column. Untagged struct-typed fields are
// walked recursively, so a row can be modelled as nested groups (for example
// the exam configuration's categories) while the table stays flat.
// Fields tagged `db:"-"` are ignored.

type fieldRef struct {
	column string
	index  []int
}

var fieldCache sync.Map // reflect.Type -> []fieldRef

func fieldsOf(t reflect.Type) []fieldRef {
	if cached, ok := fieldCache.Load(t); ok {
		return cached.([]fieldRef)
	}
	var refs []fieldRef
	collectFields(t, nil, &refs)
	fieldCache.Store(t, refs)
	return refs
}

var timeType = reflect.TypeFor[time.Time]()

func collectFields(t reflect.Type, prefix []int, out *[]fieldRef) {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		index := append(append([]int(nil), prefix...), i)
		tag := f.Tag.Get("db")
		switch {
		case tag == "-":
			continue
		case tag != "":
			*out = append(*out, fieldRef{column: tag, index: index})
		case f.Type.Kind() == reflect.Struct && f.Type != timeType:
			collectFields(f.Type, index, out)
		}
	}
}

func structValue(v any) reflect.Value {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("database: expected pointer to struct, got %T", v))
	}
	return rv.Elem()
}

// Columns lists the mapped column names of the struct pointed to by v, in
// declaration order, skipping any listed in exclude.
func Columns(v any, exclude ...string) []string {
	refs := fieldsOf(structValue(v).Type())
	cols := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(exclude, ref.column) {
			cols = append(cols, ref.column)
		}
	}
	return cols
}

// ScanTargets returns pointers to v's mapped fields, aligned with Columns(v).
func ScanTargets(v any, exclude ...string) []any {
	sv := structValue(v)
	refs := fieldsOf(sv.Type())
	ptrs := make([]any, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(exclude, ref.column) {
			ptrs = append(ptrs, sv.FieldByIndex(ref.index).Addr().Interface())
		}
	}
	return ptrs
}

// Values returns v's mapped field values, aligned with Columns(v).
func Values(v any, exclude ...string) []any {
	sv := structValue(v)
	refs := fieldsOf(sv.Type())
	vals := make([]any, 0, len(refs))
	for _, ref := range refs {
		if !slices.Contains(exclude, ref.column) {
			vals = append(vals, sv.FieldByIndex(ref.index).Interface())
		}
	}
	return vals
}

// Placeholders renders "$start, $start+1, ..." for n parameters.
func Placeholders(start, n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "$%d", start+i)
	}
	return b.String()
}

// SetClause renders "col1 = $start, col2 = $start+1, ...".
func SetClause(cols []string, start int) string {
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s = $%d", c, start+i)
	}
	return b.String()
}
