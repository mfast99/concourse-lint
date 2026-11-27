package lint

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/config"
)

// PathOptions control CheckPaths.
type PathOptions struct {
	// ConfigPath is a config file for every input (--config); "" searches
	// the nearest one per file (spec/README.md, "Config file").
	ConfigPath string
	// Stdin is read for the path "-", which is reported as StdinName.
	Stdin     io.Reader
	StdinName string
	// Allow, when set, is asked before a file is read. A file it refuses is
	// not read and becomes an input error with its message (the MCP server
	// keeps reads inside the working directory).
	Allow func(path string) error
}

// PathsResult is what CheckPaths found, in the order of the paths.
type PathsResult struct {
	Files []CheckedFile
	// Inputs that could not be read. The other files were still checked.
	Errors []InputError
}

// CheckedFile holds the findings of one checked input.
type CheckedFile struct {
	// As given, joined with the folder for a file found in one; StdinName
	// for "-".
	Path     string
	Findings []analysis.Finding
}

// InputError is an input that could not be read, e.g. a missing file.
type InputError struct {
	Path    string
	Message string
}

// CheckPaths checks files and folders as the CLI does (spec/README.md, "CLI
// contract"): a folder gives every pipeline in it, recursively and sorted;
// a file given by name is always checked; every file gets its config, and
// excluded files are skipped. A broken config is an error, and then nothing
// is checked: silently falling back to the defaults would be dangerous in
// CI.
func CheckPaths(paths []string, options PathOptions) (PathsResult, error) {
	var result PathsResult
	inputs := expandFolders(paths, &result)
	configs, err := loadConfigs(inputs, options.ConfigPath)
	if err != nil {
		return PathsResult{}, err
	}
	for _, input := range inputs {
		config := configs[input.path]
		if input.path != "-" && config.Excluded(input.path) {
			continue
		}
		displayPath, source, err := readInput(input.path, options)
		if err != nil {
			result.Errors = append(result.Errors, InputError{Path: displayPath, Message: err.Error()})
			continue
		}
		if input.fromFolder && (!LooksLikePipeline(source) || isActiveTemplate(source, config)) {
			continue
		}
		result.Files = append(result.Files, CheckedFile{Path: displayPath, Findings: CheckWithConfig(displayPath, source, config)})
	}
	return result, nil
}

// input is one file to check.
type input struct {
	path string
	// fromFolder: found in a folder, so it is skipped if it is no pipeline.
	fromFolder bool
}

// expandFolders replaces every folder by the *.yml and *.yaml files in it,
// recursively and sorted, skipping folders whose name starts with a dot
// (.git, .github) and symlinks whose target is missing (a submodule not
// checked out, an editor lock file). A folder that cannot be read becomes an
// input error.
func expandFolders(paths []string, result *PathsResult) []input {
	var inputs []input
	for _, path := range paths {
		info, err := os.Stat(path)
		if path == "-" || err != nil || !info.IsDir() {
			inputs = append(inputs, input{path: path})
			continue
		}
		walkErr := filepath.WalkDir(path, func(walkedPath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if walkedPath != path && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if extension := filepath.Ext(walkedPath); extension == ".yml" || extension == ".yaml" {
				if isDanglingSymlink(walkedPath, entry) {
					return nil
				}
				inputs = append(inputs, input{path: walkedPath, fromFolder: true})
			}
			return nil
		})
		if walkErr != nil {
			result.Errors = append(result.Errors, InputError{Path: path, Message: walkErr.Error()})
		}
	}
	return inputs
}

// loadConfigs returns the config of every input: the file from explicitPath
// for all of them, or the one the search finds (stdin: from the current
// folder). Configs are loaded once per folder.
func loadConfigs(inputs []input, explicitPath string) (map[string]*config.Config, error) {
	configs := map[string]*config.Config{}
	if explicitPath != "" {
		explicit, err := LoadConfig(explicitPath)
		if err != nil {
			return nil, err
		}
		for _, input := range inputs {
			configs[input.path] = explicit
		}
		return configs, nil
	}
	byFolder := map[string]*config.Config{}
	for _, input := range inputs {
		folder := filepath.Dir(input.path)
		if input.path == "-" {
			folder = "."
		}
		loaded, seen := byFolder[folder]
		if !seen {
			var err error
			loaded, err = ConfigFor(filepath.Join(folder, "pipeline.yml"))
			if err != nil {
				return nil, err
			}
			byFolder[folder] = loaded
		}
		configs[input.path] = loaded
	}
	return configs, nil
}

// readInput reads a file, or stdin for "-", after asking options.Allow. It
// returns the name under which the input is reported.
func readInput(path string, options PathOptions) (displayPath string, source []byte, err error) {
	if path == "-" {
		if options.Stdin == nil {
			return options.StdinName, nil, os.ErrInvalid
		}
		source, err = io.ReadAll(options.Stdin)
		return options.StdinName, source, err
	}
	if options.Allow != nil {
		if err := options.Allow(path); err != nil {
			return path, nil, err
		}
	}
	source, err = os.ReadFile(path)
	return path, source, err
}

// isDanglingSymlink reports whether the folder entry is a symlink whose
// target does not exist.
func isDanglingSymlink(path string, entry fs.DirEntry) bool {
	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}
