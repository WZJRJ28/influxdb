package reads

//go:generate env GO111MODULE=on go run github.com/benbjohnson/tmpl -data=@types.tmpldata table.gen.go.tmpl

import (
	"sync/atomic"

	"github.com/apache/arrow/go/arrow/array"
	"github.com/influxdata/flux"
	"github.com/influxdata/flux/arrow"
	"github.com/influxdata/flux/execute"
	"github.com/influxdata/flux/memory"
	"github.com/influxdata/influxdb/models"
)

type table struct {
	bounds execute.Bounds
	key    flux.GroupKey
	cols   []flux.ColMeta

	// cache of the tags on the current series.
	// len(tags) == len(colMeta)
	tags [][]byte
	defs [][]byte

	done chan struct{}

	// The current number of records in memory
	l int

	colBufs []array.Interface

	err error

	cancelled int32
	alloc     *memory.Allocator
}

func newTable(
	done chan struct{},
	bounds execute.Bounds,
	key flux.GroupKey,
	cols []flux.ColMeta,
	defs [][]byte,
	alloc *memory.Allocator,
) table {
	return table{
		done:    done,
		bounds:  bounds,
		key:     key,
		tags:    make([][]byte, len(cols)),
		defs:    defs,
		colBufs: make([]array.Interface, len(cols)),
		cols:    cols,
		alloc:   alloc,
	}
}

func (t *table) Key() flux.GroupKey   { return t.key }
func (t *table) Cols() []flux.ColMeta { return t.cols }
func (t *table) Err() error           { return t.err }
func (t *table) Empty() bool          { return t.l == 0 }
func (t *table) Len() int             { return t.l }

func (t *table) Cancel() {
	atomic.StoreInt32(&t.cancelled, 1)
}

func (t *table) isCancelled() bool {
	return atomic.LoadInt32(&t.cancelled) != 0
}

func (t *table) Retain()  {}
func (t *table) Release() {}

func (t *table) Bools(j int) *array.Boolean {
	execute.CheckColType(t.cols[j], flux.TBool)
	return t.colBufs[j].(*array.Boolean)
}

func (t *table) Ints(j int) *array.Int64 {
	execute.CheckColType(t.cols[j], flux.TInt)
	return t.colBufs[j].(*array.Int64)
}

func (t *table) UInts(j int) *array.Uint64 {
	execute.CheckColType(t.cols[j], flux.TUInt)
	return t.colBufs[j].(*array.Uint64)
}

func (t *table) Floats(j int) *array.Float64 {
	execute.CheckColType(t.cols[j], flux.TFloat)
	return t.colBufs[j].(*array.Float64)
}

func (t *table) Strings(j int) *array.Binary {
	execute.CheckColType(t.cols[j], flux.TString)
	return t.colBufs[j].(*array.Binary)
}

func (t *table) Times(j int) *array.Int64 {
	execute.CheckColType(t.cols[j], flux.TTime)
	return t.colBufs[j].(*array.Int64)
}

// readTags populates b.tags with the provided tags
func (t *table) readTags(tags models.Tags) {
	for j := range t.tags {
		t.tags[j] = t.defs[j]
	}

	if len(tags) == 0 {
		return
	}

	for _, tag := range tags {
		j := execute.ColIdx(string(tag.Key), t.cols)
		t.tags[j] = tag.Value
	}
}

// appendTags fills the colBufs for the tag columns with the tag value.
func (t *table) appendTags() {
	for j := range t.cols {
		v := t.tags[j]
		if v != nil {
			b := arrow.NewStringBuilder(t.alloc)
			b.Reserve(t.l)
			b.ReserveData(t.l * len(v))
			for i := 0; i < t.l; i++ {
				b.Append(v)
			}
			t.colBufs[j] = b.NewArray()
			b.Release()
		}
	}
}

// appendBounds fills the colBufs for the time bounds
func (t *table) appendBounds() {
	bounds := []execute.Time{t.bounds.Start, t.bounds.Stop}
	for j := range []int{startColIdx, stopColIdx} {
		b := arrow.NewIntBuilder(t.alloc)
		b.Reserve(t.l)
		for i := 0; i < t.l; i++ {
			b.UnsafeAppend(int64(bounds[j]))
		}
		t.colBufs[j] = b.NewArray()
		b.Release()
	}
}

func (t *table) closeDone() {
	if t.done != nil {
		close(t.done)
		t.done = nil
	}
}

func (t *floatTable) toArrowBuffer(vs []float64) *array.Float64 {
	return arrow.NewFloat(vs, t.alloc)
}
func (t *floatGroupTable) toArrowBuffer(vs []float64) *array.Float64 {
	return arrow.NewFloat(vs, t.alloc)
}
func (t *integerTable) toArrowBuffer(vs []int64) *array.Int64 {
	return arrow.NewInt(vs, t.alloc)
}
func (t *integerGroupTable) toArrowBuffer(vs []int64) *array.Int64 {
	return arrow.NewInt(vs, t.alloc)
}
func (t *unsignedTable) toArrowBuffer(vs []uint64) *array.Uint64 {
	return arrow.NewUint(vs, t.alloc)
}
func (t *unsignedGroupTable) toArrowBuffer(vs []uint64) *array.Uint64 {
	return arrow.NewUint(vs, t.alloc)
}
func (t *stringTable) toArrowBuffer(vs []string) *array.Binary {
	return arrow.NewString(vs, t.alloc)
}
func (t *stringGroupTable) toArrowBuffer(vs []string) *array.Binary {
	return arrow.NewString(vs, t.alloc)
}
func (t *booleanTable) toArrowBuffer(vs []bool) *array.Boolean {
	return arrow.NewBool(vs, t.alloc)
}
func (t *booleanGroupTable) toArrowBuffer(vs []bool) *array.Boolean {
	return arrow.NewBool(vs, t.alloc)
}
