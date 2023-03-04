package azure

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/parallel"
	"github.com/influxdata/telegraf/plugins/processors"
	"github.com/patrickmn/go-cache"
	"github.com/tuscanylabs/telegraf-processor-azure-imds/internal/imds"
	"time"
)

//go:embed sample.conf
var sampleConfig string

type AzureIMDSProcessor struct {
	ImdsTags         []string        `toml:"imds_tags"`
	Timeout          config.Duration `toml:"timeout"`
	Ordered          bool            `toml:"ordered"`
	MaxParallelCalls int             `toml:"max_parallel_calls"`
	Log              telegraf.Logger `toml:"-"`

	imdsClient  *imds.Client
	imdsTagsMap map[string]struct{}
	parallel    parallel.Parallel
	cache       *cache.Cache
}

const (
	DefaultMaxOrderedQueueSize = 10_000
	DefaultMaxParallelCalls    = 10
	DefaultTimeout             = 10 * time.Second
)

var allowedImdsTags = map[string]struct{}{
	"azEnvironment":     {},
	"location":          {},
	"placementGroupId":  {},
	"resourceGroupName": {},
	"resourceId":        {},
	"subscriptionId":    {},
	"version":           {},
	"vmid":              {},
	"zone":              {},
}

func (*AzureIMDSProcessor) SampleConfig() string {
	return sampleConfig
}

func (r *AzureIMDSProcessor) Add(metric telegraf.Metric, _ telegraf.Accumulator) error {
	r.parallel.Enqueue(metric)
	return nil
}

func (r *AzureIMDSProcessor) Init() error {
	r.Log.Debug("Initializing Azure IMDS Processor")
	if len(r.ImdsTags) == 0 {
		return errors.New("no tags specified in configuration")
	}

	for _, tag := range r.ImdsTags {
		if len(tag) == 0 || !isImdsTagAllowed(tag) {
			return fmt.Errorf("not allowed metadata tag specified in configuration: %s", tag)
		}
		r.imdsTagsMap[tag] = struct{}{}
	}
	if len(r.imdsTagsMap) == 0 {
		return errors.New("no allowed metadata tags specified in configuration")
	}

	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes.
	//
	// Cache will prevent hammering of the IMDS url which can result in throttling and unnecessary HTTP traffic which
	// may be detected by instrumentation tools such as Pixie
	r.cache = cache.New(5*time.Minute, 10*time.Minute)

	return nil
}

func (r *AzureIMDSProcessor) Start(acc telegraf.Accumulator) error {
	r.imdsClient = imds.NewClient()

	if r.Ordered {
		r.parallel = parallel.NewOrdered(acc, r.asyncAdd, DefaultMaxOrderedQueueSize, r.MaxParallelCalls)
	} else {
		r.parallel = parallel.NewUnordered(acc, r.asyncAdd, r.MaxParallelCalls)
	}

	return nil
}

func (r *AzureIMDSProcessor) Stop() {
	if r.parallel != nil {
		r.parallel.Stop()
	}
}

func (r *AzureIMDSProcessor) asyncAdd(metric telegraf.Metric) []telegraf.Metric {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.Timeout))
	defer cancel()

	// Add IMDS Instance Identity Document tags.
	if len(r.imdsTagsMap) > 0 {
		iido, err := r.imdsClient.GetInstanceMetadata(
			ctx,
			&imds.GetInstanceMetadataInput{},
		)
		if err != nil {
			r.Log.Errorf("Error when calling GetInstanceMetadata: %v", err)
			return []telegraf.Metric{metric}
		}

		for tag := range r.imdsTagsMap {
			r.Log.Infof("Getting information for tag: %s", tag)
			finding, found := r.cache.Get(tag)
			if found {
				metric.AddTag(tag, finding.(string))
			} else {
				if v := getTagFromInstanceIdentityDocument(iido, tag); v != "" {
					metric.AddTag(tag, v)
					r.cache.Set(tag, v, cache.DefaultExpiration)
				}
			}
		}
	}

	return []telegraf.Metric{metric}
}

func init() {
	processors.AddStreaming("azure_imds", func() telegraf.StreamingProcessor {
		return newAzureIMDSProcessor()
	})
}

func newAzureIMDSProcessor() *AzureIMDSProcessor {
	return &AzureIMDSProcessor{
		MaxParallelCalls: DefaultMaxParallelCalls,
		Timeout:          config.Duration(DefaultTimeout),
		imdsTagsMap:      make(map[string]struct{}),
	}
}

func getTagFromInstanceIdentityDocument(o *imds.GetMetadataInstanceOutput, tag string) string {
	switch tag {
	case "azEnvironment":
		return o.AzEnvironment
	case "location":
		return o.Location
	case "placementGroupId":
		return o.PlacementGroupID
	case "resourceGroupName":
		return o.ResourceGroupName
	case "resourceId":
		return o.ResourceID
	case "subscriptionId":
		return o.SubscriptionID
	case "version":
		return o.Version
	case "vmid":
		return o.VMID
	case "zone":
		return o.Zone
	default:
		return ""
	}
}

func isImdsTagAllowed(tag string) bool {
	_, ok := allowedImdsTags[tag]
	return ok
}
