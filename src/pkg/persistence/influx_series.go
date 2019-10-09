package persistence

import (
	"context"
	"strings"

	"github.com/influxdata/influxdb/query"
	"github.com/influxdata/influxql"
	"github.com/prometheus/prometheus/pkg/labels"
)

// TODO - can we make this a type?
//type SeriesSet map[uint64]labels.Labels

func (t *InfluxAdapter) GetSeriesSet(measurementName string, start, end int64, filterCondition influxql.Expr) (map[uint64]labels.Labels, error) {
	shardIds := t.forTimestampRange(start, end)

	var seriesIterators []query.Iterator
	for _, shardId := range shardIds {
		iterator, err := t.createSeriesIterator(shardId, measurementName, start, end, filterCondition)
		if err != nil {
			return nil, err
		}
		seriesIterators = append(seriesIterators, iterator)
	}

	seriesIterator := query.NewParallelMergeIterator(seriesIterators, query.IteratorOptions{}, len(seriesIterators))
	if seriesIterator == nil {
		return nil, nil
	}

	defer func() {
		seriesIterator.Close()
		for _, i := range seriesIterators {
			i.Close()
		}
	}()

	seriesSet := make(map[uint64]labels.Labels)
	switch typedSeriesIterator := seriesIterator.(type) {
	case query.FloatIterator:
		for {
			series, err := typedSeriesIterator.Next()
			if err != nil {
				return nil, err
			}
			if series == nil {
				break
			}

			seriesLabels := labels.NewBuilder(nil)
			for i, label := range strings.Split(series.Aux[0].(string), ",") {
				if i == 0 {
					continue
				}
				labelTuple := strings.Split(label, "=")
				seriesLabels.Set(labelTuple[0], labelTuple[1])
			}

			sl := seriesLabels.Labels()
			seriesSet[sl.Hash()] = sl
		}
	// TODO - should we bail out here? is this defined?
	default:
		// fall through
	}

	return seriesSet, nil
}

func (t *InfluxAdapter) createSeriesIterator(shardId uint64, measurementName string, start, end int64, filterCondition influxql.Expr) (query.Iterator, error) {
	shards := t.influx.ShardGroup([]uint64{shardId})

	measurementNameFilter := influxql.BinaryExpr{
		LHS: &influxql.VarRef{Val: "_name"},
		RHS: &influxql.StringLiteral{Val: measurementName},
		Op:  influxql.EQ,
	}

	var filterWithName influxql.Expr
	if filterCondition != nil {
		filterWithName = &influxql.BinaryExpr{
			LHS: &measurementNameFilter,
			RHS: filterCondition,
			Op:  influxql.AND,
		}
	} else {
		filterWithName = &measurementNameFilter
	}

	queryOpts := query.IteratorOptions{
		Aux:       []influxql.VarRef{{Val: "key", Type: influxql.String}},
		Condition: filterWithName,
		Limit:     0,
	}

	return shards.CreateIterator(
		context.Background(),
		&influxql.Measurement{SystemIterator: "_series"},
		queryOpts,
	)
}
