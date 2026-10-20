package kubernetes_test

import (
	"testing"

	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
	"github.com/stretchr/testify/assert"
)

type labelLookup struct {
	labels map[string]map[string]string
	err    error
	calls  int
}

func (l *labelLookup) get(name string) (map[string]string, error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return l.labels[name], nil
}

func TestNamespaceFilterEmptySelectorSelectsEverything(t *testing.T) {
	lookup := &labelLookup{}
	f, err := kubernetes.NewNamespaceFilter("", lookup.get, 0)
	assert.NoError(t, err)
	assert.True(t, f.Selected("anything"))
	assert.Equal(t, 0, lookup.calls)
}

func TestNamespaceFilterMatchesLabelSelector(t *testing.T) {
	lookup := &labelLookup{labels: map[string]map[string]string{
		"opendesk": {"devguard": "enabled"},
		"other":    {"devguard": "disabled"},
		"bare":     nil,
	}}
	f, err := kubernetes.NewNamespaceFilter("devguard=enabled", lookup.get, 0)
	assert.NoError(t, err)

	assert.True(t, f.Selected("opendesk"))
	assert.False(t, f.Selected("other"))
	assert.False(t, f.Selected("bare"))
}
