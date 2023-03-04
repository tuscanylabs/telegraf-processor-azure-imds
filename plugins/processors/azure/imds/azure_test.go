package azure

import (
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

func TestBasicStartup(t *testing.T) {
	p := newAzureIMDSProcessor()
	p.Log = &testutil.Logger{}
	p.ImdsTags = []string{"location", "zone"}
	acc := &testutil.Accumulator{}
	require.NoError(t, p.Init())

	require.Len(t, acc.GetTelegrafMetrics(), 0)
	require.Len(t, acc.Errors, 0)
}
