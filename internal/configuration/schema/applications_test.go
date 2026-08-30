package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestApplicationsConfigurationDecodes(t *testing.T) {
	var configuration Configuration

	err := yaml.Unmarshal([]byte(`
applications:
  - slug: grafana
    name: Grafana
  - slug: disabled
    name: Disabled
    domain: disabled.example.com
    group: " team, odd "
    enabled: false
`), &configuration)
	require.NoError(t, err)
	require.Len(t, configuration.Applications, 2)
	assert.Equal(t, "grafana", configuration.Applications[0].Slug)
	assert.True(t, configuration.Applications[0].IsEnabled())
	assert.Equal(t, " team, odd ", configuration.Applications[1].Group)
	assert.False(t, configuration.Applications[1].IsEnabled())
}
