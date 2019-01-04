package dockercompose

import "testing"

// ParseConfig must return services topologically sorted wrt dependencies.
func TestParseConfigPreservesServiceOrder(t *testing.T) {
	dcc := NewFakeDockerComposeClient(t)

}
