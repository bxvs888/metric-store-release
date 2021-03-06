package storage

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/cloudfoundry/metric-store-release/src/internal/routing"
	"github.com/prometheus/prometheus/pkg/labels"
	prom_storage "github.com/prometheus/prometheus/storage"
)

type ReplicatedQuerier struct {
	store      prom_storage.Storage
	lookup     routing.Lookup
	localIndex int
	queriers   []prom_storage.Querier
}

func NewReplicatedQuerier(
	localStore prom_storage.Storage,
	localIndex int,
	queriers []prom_storage.Querier,
	lookup routing.Lookup,
) *ReplicatedQuerier {
	return &ReplicatedQuerier{
		store:      localStore,
		localIndex: localIndex,
		queriers:   queriers,
		lookup:     lookup,
	}
}

// TODO: add test for `{source_id="foo"}`
func (r *ReplicatedQuerier) Select(params *prom_storage.SelectParams, matchers ...*labels.Matcher) (prom_storage.SeriesSet, prom_storage.Warnings, error) {
	var err error
	var metricName string

	clients := routing.NewClients(r.lookup, r.localIndex)

	for _, matcher := range matchers {
		if matcher.Name == labels.MetricName {
			if matcher.Type != labels.MatchEqual {
				return nil, nil, errors.New("only strict equality is supported for metric names")
			}
			metricName = matcher.Value
		}
	}
	// TODO: no metric name, return an error?
	clientsIndeciesWithMetric, metricContainedLocally := clients.MetricDistribution(metricName)

	// TODO: does Select(...) even get this type of call any more?
	// maybe? "{source_id: '...'}"
	// if len(metricNames) == 0 && len(clientsWithAllMetrics) == 0 {
	// 	return erp.engine.InstantQuery(ctx, req, erp.localReader)
	// }

	if metricContainedLocally {
		localQuerier, err := r.store.Querier(context.Background(), 0, 0)
		if err != nil {
			return nil, nil, err
		}

		return localQuerier.Select(params, matchers...)
	}

	routing.Shuffle(clientsIndeciesWithMetric)

	for _, index := range clientsIndeciesWithMetric {
		remoteQuerier := r.queriers[index]
		return remoteQuerier.Select(params, matchers...)
	}

	return nil, nil, err
}

func (r *ReplicatedQuerier) LabelNames() ([]string, prom_storage.Warnings, error) {
	labelNamesMap := make(map[string]struct{})

	for _, querier := range r.queriers {
		labelNames, _, _ := querier.LabelNames()
		for _, labelName := range labelNames {
			labelNamesMap[labelName] = struct{}{}
		}
	}

	allLabelNames := make([]string, 0, len(labelNamesMap))
	for name := range labelNamesMap {
		allLabelNames = append(allLabelNames, name)
	}
	sort.Strings(allLabelNames)

	return allLabelNames, nil, nil
}

func (r *ReplicatedQuerier) LabelValues(name string) ([]string, prom_storage.Warnings, error) {
	var results [][]string

	for _, querier := range r.queriers {
		labelValues, _, _ := querier.LabelValues(name)
		results = append(results, labelValues)
	}

	return mergeStringSlices(results), nil, nil
}

func mergeStringSlices(ss [][]string) []string {
	switch len(ss) {
	case 0:
		return nil
	case 1:
		return ss[0]
	case 2:
		return mergeTwoStringSlices(ss[0], ss[1])
	default:
		halfway := len(ss) / 2
		return mergeTwoStringSlices(
			mergeStringSlices(ss[:halfway]),
			mergeStringSlices(ss[halfway:]),
		)
	}
}

func mergeTwoStringSlices(a, b []string) []string {
	i, j := 0, 0
	result := make([]string, 0, len(a)+len(b))
	for i < len(a) && j < len(b) {
		switch strings.Compare(a[i], b[j]) {
		case 0:
			result = append(result, a[i])
			i++
			j++
		case -1:
			result = append(result, a[i])
			i++
		case 1:
			result = append(result, b[j])
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func (r *ReplicatedQuerier) Close() error {
	return nil
}
