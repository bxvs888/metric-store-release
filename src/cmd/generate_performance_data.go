package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/metric-store-release/src/pkg/persistence"
	shared "github.com/cloudfoundry/metric-store-release/src/pkg/testing"
	"github.com/prometheus/prometheus/pkg/labels"
)

func main() {
	numberOfPoints := 1000000

	if len(os.Args) == 1 {
		panic("You must provide the storage path as an arg")
	}

	fmt.Println(" - storing data in", os.Args[1])

	spyPersistentStoreMetrics := shared.NewSpyMetricRegistrar()
	persistentStore := persistence.NewStore(
		os.Args[1],
		spyPersistentStoreMetrics,
	)

	appender, _ := persistentStore.Appender()

	type label struct {
		Name  string
		Value []string
	}

	testLabels := []label{
		{Name: labels.MetricName, Value: []string{"bigmetric"}},
		{Name: "app_id", Value: []string{"bde5831e-a819-4a34-9a46-012fd2e821e6b", "!!!!!"}},
		{Name: "app_name", Value: []string{"bblog"}},
		{Name: "bosh_environment", Value: []string{"vpc-bosh-run-pivotal-io"}},
		{Name: "deployment", Value: []string{"pws-diego-cellblock-09"}},
		{Name: "index", Value: []string{"9b74a5b1-9af9-4715-a57a-bd28ad7e7f1b"}},
		{Name: "instance_id", Value: []string{"0"}},
		{Name: "ip", Value: []string{"10.10.148.146"}},
		{Name: "job", Value: []string{"diego-cell"}},
		{Name: "organization_id", Value: []string{"ab2de77b-a484-4690-9201-b8eaf707fd87"}},
		{Name: "organization_name", Value: []string{"blars"}},
		{Name: "origin", Value: []string{"rep"}},
		{Name: "process_id", Value: []string{"328de02b-79f1-4f8d-b3b2-b81112809603"}},
		{Name: "process_instance_id", Value: []string{"2652349b-4d40-4b51-4165-7129"}},
		{Name: "process_type", Value: []string{"web"}},
		{Name: "source_id", Value: []string{"5ee5831e-a819-4a34-9a46-012fd2e821e7"}},
		{Name: "space_id", Value: []string{"eb94778d-66b5-4804-abcb-e9efd7f725aa"}},
		{Name: "space_name", Value: []string{"bblog"}},
		{Name: "unit", Value: []string{"percentage"}},
		{Name: "uri", Value: []string{"https://google.ca"}},
	}

	fmt.Printf(" - generating %d points\n", numberOfPoints)
	var tl labels.Labels
	for i := 0; i < numberOfPoints; i++ {
		tl = labels.Labels{}
		for _, l := range testLabels {
			tl = append(tl, labels.Label{
				Name:  l.Name,
				Value: l.Value[i%len(l.Value)],
			})
		}
		appender.Add(tl, time.Now().Add(time.Duration(-i)*time.Second).UnixNano(), .5)
	}
	appender.Commit()

	fmt.Println(" - compacting data")
	persistentStore.Compact()

	fmt.Println(" - waiting on compaction")
	time.Sleep(60 * time.Second)

	fmt.Println(" - DONE!")
}
