// Copyright 2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"bytes"
	"github.com/dolthub/go-mysql-server/sql"
	"math"
	"sort"

	"github.com/dolthub/dolt/go/store/prolly"

	"github.com/dolthub/dolt/go/libraries/doltcore/table/typed/noms"
	"github.com/dolthub/dolt/go/store/types"
)

func ClosedRange(tpl1, tpl2 types.Tuple) *noms.ReadRange {
	return CustomRange(tpl1, tpl2, sql.Closed, sql.Closed)
}

func OpenRange(tpl1, tpl2 types.Tuple) *noms.ReadRange {
	return CustomRange(tpl1, tpl2, sql.Open, sql.Open)
}

func CustomRange(tpl1, tpl2 types.Tuple, bt1, bt2 sql.RangeBoundType) *noms.ReadRange {
	var nrc nomsRangeCheck
	_ = tpl1.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		if bt1 == sql.Closed {
			nrc = append(nrc, columnBounds{
				boundsCase: boundsCase_greaterEquals_infinity,
				lowerbound: tupleVal,
			})
		} else {
			nrc = append(nrc, columnBounds{
				boundsCase: boundsCase_greater_infinity,
				lowerbound: tupleVal,
			})
		}
		return false, nil
	})
	_ = tpl2.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		idx := (tupleIndex - 1) / 2
		if bt2 == sql.Closed {
			// Bounds cases are enum aliases on bytes, and they're arranged such that we can increment the case
			// that was previously set when evaluating the lowerbound to get the proper overall case.
			nrc[idx].boundsCase += 1
			nrc[idx].upperbound = tupleVal
		} else {
			nrc[idx].boundsCase += 2
			nrc[idx].upperbound = tupleVal
		}
		return false, nil
	})
	return &noms.ReadRange{
		Start:     tpl1,
		Inclusive: true,
		Reverse:   false,
		Check:     nrc,
	}
}

func GreaterThanRange(tpl types.Tuple) *noms.ReadRange {
	var nrc nomsRangeCheck
	_ = tpl.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		nrc = append(nrc, columnBounds{
			boundsCase: boundsCase_greater_infinity,
			lowerbound: tupleVal,
		})
		return false, nil
	})
	return &noms.ReadRange{
		Start:     tpl,
		Inclusive: true,
		Reverse:   false,
		Check:     nrc,
	}
}

func LessThanRange(tpl types.Tuple) *noms.ReadRange {
	var nrc nomsRangeCheck
	_ = tpl.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		nrc = append(nrc, columnBounds{
			boundsCase: boundsCase_infinity_less,
			upperbound: tupleVal,
		})
		return false, nil
	})
	return &noms.ReadRange{
		Start:     types.EmptyTuple(types.Format_Default),
		Inclusive: true,
		Reverse:   false,
		Check:     nrc,
	}
}

func GreaterOrEqualRange(tpl types.Tuple) *noms.ReadRange {
	var nrc nomsRangeCheck
	_ = tpl.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		nrc = append(nrc, columnBounds{
			boundsCase: boundsCase_greaterEquals_infinity,
			lowerbound: tupleVal,
		})
		return false, nil
	})
	return &noms.ReadRange{
		Start:     tpl,
		Inclusive: true,
		Reverse:   false,
		Check:     nrc,
	}
}

func LessOrEqualRange(tpl types.Tuple) *noms.ReadRange {
	var nrc nomsRangeCheck
	_ = tpl.IterFields(func(tupleIndex uint64, tupleVal types.Value) (stop bool, err error) {
		if tupleIndex%2 == 0 {
			return false, nil
		}
		nrc = append(nrc, columnBounds{
			boundsCase: boundsCase_infinity_lessEquals,
			upperbound: tupleVal,
		})
		return false, nil
	})
	return &noms.ReadRange{
		Start:     types.EmptyTuple(types.Format_Default),
		Inclusive: true,
		Reverse:   false,
		Check:     nrc,
	}
}

func NullRange() *noms.ReadRange {
	return &noms.ReadRange{
		Start:     types.EmptyTuple(types.Format_Default),
		Inclusive: true,
		Reverse:   false,
		Check: nomsRangeCheck{
			{
				boundsCase: boundsCase_isNull,
			},
		},
	}
}

func NotNullRange() *noms.ReadRange {
	return &noms.ReadRange{
		Start:     types.EmptyTuple(types.Format_Default),
		Inclusive: true,
		Reverse:   false,
		Check: nomsRangeCheck{
			{
				boundsCase: boundsCase_infinity_infinity,
			},
		},
	}
}

func AllRange() *noms.ReadRange {
	return &noms.ReadRange{
		Start:     types.EmptyTuple(types.Format_Default),
		Inclusive: true,
		Reverse:   false,
		Check:     nomsRangeCheck{},
	}
}

func ReadRangesEqual(nr1, nr2 *noms.ReadRange) bool {
	if nr1 == nil || nr2 == nil {
		if nr1 == nil && nr2 == nil {
			return true
		}
		return false
	}
	if nr1.Inclusive != nr2.Inclusive || nr1.Reverse != nr2.Reverse || !nr1.Start.Equals(nr2.Start) ||
		!nr1.Check.(nomsRangeCheck).Equals(nr2.Check.(nomsRangeCheck)) {
		return false
	}
	return true
}

func NomsRangesFromIndexLookup(ctx *sql.Context, lookup sql.IndexLookup) ([]*noms.ReadRange, error) {
	return lookup.Index.(*doltIndex).nomsRanges(ctx, lookup.Ranges...)
}

func ProllyRangesFromIndexLookup(ctx *sql.Context, lookup sql.IndexLookup) ([]prolly.Range, error) {
	idx := lookup.Index.(*doltIndex)
	return idx.prollyRanges(ctx, idx.ns, lookup.Ranges...)
}

func DoltIndexFromSqlIndex(idx sql.Index) DoltIndex {
	return idx.(DoltIndex)
}

func LexFloat(f float64) uint64 {
	b := math.Float64bits(f)
	if b >> 63 == 1 {
		tmp := math.Float64bits(math.MaxFloat64)
		b = tmp - b
	}
	b = b ^ (1 << 63) // flip the sign bit
	return b
}

func UnLexFloat(b uint64) float64 {
	if b >> 63 == 0 {
		b = math.Float64bits(math.MaxFloat64) - b
	}
	b = b ^ (1 << 63) // flip the sign bit
	if b == math.MaxInt64 {
		return math.Inf(-1)
	}
	return math.Float64frombits(b)
}

func ZValue(p sql.Point) [16]byte {
	xLex := LexFloat(p.X)
	yLex := LexFloat(p.Y)

	res := [16]byte{}
	for i := 0; i < 16; i++ {
		for j := 0; j < 4; j++ {
			x, y := byte((xLex&1) << 1), byte(yLex&1)
			res[15-i] |= (x | y) << (2 * j)
			xLex, yLex = xLex>>1, yLex>>1
		}
	}
	return res
}

func UnZValue(z [16]byte) sql.Point {
	var x, y uint64
	for i := 15; i >= 0; i-- {
		zv := uint64(z[i])
		for j := 3; j >= 0; j-- {
			y |= (zv & 1) << (63 - (4 * i + j))
			zv >>= 1

			x |= (zv & 1) << (63 - (4 * i + j))
			zv >>= 1
		}
	}
	xf := UnLexFloat(x)
	yf := UnLexFloat(y)
	return sql.Point{X: xf, Y: yf}
}

func ZSort(points []sql.Point) []sql.Point{
	sort.Slice(points, func (i,j int) bool {
		zi, zj := ZValue(points[i]), ZValue(points[j])
		return bytes.Compare(zi[:], zj[:]) < 0
	})
	return points
}