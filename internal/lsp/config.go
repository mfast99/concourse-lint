package lsp

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/mfast99/concourse-lint/internal/config"
	"github.com/mfast99/concourse-lint/internal/lint"
)

// configCache holds the config of every folder with an open pipeline, so
// that the file is not searched and read on every key press. It is emptied
// when a config file changes on disk.
type configCache struct {
	mutex    sync.Mutex
	byFolder map[string]configResult
	// published are the config files that show an error, so that it can be
	// cleared again.
	published map[string]bool
}

type configResult struct {
	config *config.Config
	err    error
}

func newConfigCache() *configCache {
	return &configCache{byFolder: map[string]configResult{}, published: map[string]bool{}}
}

// forFile returns the config for a file, from the cache or the disk.
func (cache *configCache) forFile(path string) (*config.Config, error) {
	folder := filepath.Dir(path)
	cache.mutex.Lock()
	cached, found := cache.byFolder[folder]
	cache.mutex.Unlock()
	if found {
		return cached.config, cached.err
	}
	loaded, err := lint.ConfigFor(path)
	cache.mutex.Lock()
	cache.byFolder[folder] = configResult{config: loaded, err: err}
	cache.mutex.Unlock()
	return loaded, err
}

// forget empties the cache and returns the config files that show an error.
func (cache *configCache) forget() []string {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.byFolder = map[string]configResult{}
	var uris []string
	for uri := range cache.published {
		uris = append(uris, uri)
	}
	cache.published = map[string]bool{}
	return uris
}

// publishConfigError shows the error of a config file on disk in that file,
// unless it is open: then its own check shows the error in the text being
// edited.
func (server *server) publishConfigError(err error) {
	var configError *config.Error
	if !errors.As(err, &configError) || configError.Path == "" {
		return
	}
	uri := pathToURI(configError.Path)
	server.mutex.Lock()
	_, isOpen := server.documents[uri]
	server.mutex.Unlock()
	if isOpen {
		return
	}
	server.configs.mutex.Lock()
	server.configs.published[uri] = true
	server.configs.mutex.Unlock()
	server.publish(uri, nil, []Diagnostic{configErrorDiagnostic(configError, readLineIndex(configError.Path))})
}

// configFilesChanged is called when a config file changed on disk: every
// open document is checked again with the new config.
func (server *server) configFilesChanged() {
	for _, uri := range server.configs.forget() {
		server.publish(uri, nil, nil)
	}
	server.mutex.Lock()
	var uris []string
	for uri := range server.documents {
		uris = append(uris, uri)
	}
	server.mutex.Unlock()
	for _, uri := range uris {
		server.check(uri)
	}
}

// recheckAfterConfigEdit: when an edited config file is closed, its errors
// on disk (if any) are shown instead of the ones of the edited text.
func (server *server) recheckAfterConfigEdit(uri string) {
	if isConfigFile(uriToPath(uri)) {
		server.configFilesChanged()
	}
}

// readLineIndex reads a file from disk for the position of an error in it.
func readLineIndex(path string) lineIndex {
	content, err := os.ReadFile(path)
	if err != nil {
		return newLineIndex("")
	}
	return newLineIndex(string(content))
}
