package helm

import (
	"fmt"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
	"sigs.k8s.io/yaml"
	"sync"
)

type IndexLoader interface {
	LoadIndexFile(URL string) (*repo.IndexFile, error)
}

type DefaultProviders struct {
	Providers getter.Providers
	mu        sync.Mutex
}

func (r *DefaultProviders) getProviders() getter.Providers {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Providers != nil {
		return r.Providers
	}
	r.Providers = getter.All(cli.New())
	r.Providers = append(r.Providers, S3Provider())
	return r.Providers
}

type DirectLoader struct {
	DefaultProviders
}

func (r *DirectLoader) LoadIndexFile(URL string) (*repo.IndexFile, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid chart URL format: %s", URL)
	}

	client, err := r.getProviders().ByScheme(u.Scheme)
	if err != nil {
		return nil, fmt.Errorf("could not find protocol handler for: %s", u.Scheme)
	}
	indexURL := URL + "/index.yaml"
	content, err := client.Get(indexURL, getter.WithURL(indexURL))
	if content == nil {
		return nil, fmt.Errorf("no content for %s", indexURL)
	}
	var indexFile repo.IndexFile
	if err := yaml.UnmarshalStrict(content.Bytes(), &indexFile); err != nil {
		return nil, fmt.Errorf("failed to parse index.yaml: %w", err)
	}
	return &indexFile, nil
}

type CachedLoader struct {
	IndexLoader IndexLoader
	cache       map[string]*repo.IndexFile
	mu          sync.Mutex
}

func (r *CachedLoader) LoadIndexFile(URL string) (*repo.IndexFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		r.cache = make(map[string]*repo.IndexFile)
	}
	if indexFile, ok := r.cache[URL]; ok {
		return indexFile, nil
	}
	indexFile, err := r.IndexLoader.LoadIndexFile(URL)
	if err != nil {
		return nil, err
	}
	r.cache[URL] = indexFile
	return indexFile, nil
}

func ApplyToUpdate(r IndexLoader, update *Update) {

}
