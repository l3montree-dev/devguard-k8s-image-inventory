package kubernetes

import (
	"context"
	"log/slog"
	"sync"
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const namespaceFilterTTL = 30 * time.Second

type namespaceDecision struct {
	selected bool
	expires  time.Time
}

// NamespaceFilter decides whether the pods of a namespace are in scope, based on a
// label selector for namespaces. The pod informer listens on all namespaces, so
// the selector has to be applied to every event.
type NamespaceFilter struct {
	selector labels.Selector
	lookup   func(name string) (map[string]string, error)
	ttl      time.Duration

	mu    sync.Mutex
	cache map[string]namespaceDecision
}

func NewNamespaceFilter(selector string, lookup func(name string) (map[string]string, error), ttl time.Duration) (*NamespaceFilter, error) {
	parsed, err := labels.Parse(selector)
	if err != nil {
		return nil, err
	}

	return &NamespaceFilter{selector: parsed, lookup: lookup, ttl: ttl, cache: make(map[string]namespaceDecision)}, nil
}

// Selected reports whether the namespace matches the selector. A namespace
// which can not be looked up is not selected.
func (f *NamespaceFilter) Selected(namespace string) bool {
	if f.selector.Empty() {
		return true
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if d, ok := f.cache[namespace]; ok && time.Now().Before(d.expires) {
		return d.selected
	}

	nsLabels, err := f.lookup(namespace)
	if err != nil {
		slog.Warn("failed to look up namespace labels", "namespace", namespace, "err", err)
		return false
	}

	selected := f.selector.Matches(labels.Set(nsLabels))
	f.cache[namespace] = namespaceDecision{selected: selected, expires: time.Now().Add(f.ttl)}

	return selected
}

func (client *KubeClient) NewNamespaceFilter(selector string) (*NamespaceFilter, error) {
	return NewNamespaceFilter(selector, func(name string) (map[string]string, error) {
		ns, err := client.Client.Client.CoreV1().Namespaces().Get(context.Background(), name, meta.GetOptions{})
		if err != nil {
			return nil, err
		}

		return ns.Labels, nil
	}, namespaceFilterTTL)
}
