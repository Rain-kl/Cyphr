// Package config manages application paths, environment loading, and python runtime detection.
package config

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultPythonCmd = "python"
	python3Cmd       = "python3"
	envKeyValueParts = 2
)

// AppPaths holds all relevant filesystem paths for the agent and installer.
type AppPaths struct {
	RootDir          string
	AgentDir         string
	PidFile          string
	LogFile          string
	ConfigFile       string
	ConfigExample    string
	EnvFile          string
	DownloadPidFile  string
	DownloadLogFile  string
	DownloadInfoFile string
	ModelsDir        string
	PythonBin        string
}

// FindAgentRoot locates the backend/agent root directory based on the executable or current working directory.
func FindAgentRoot() string {
	// 1. Check CYPHR_AGENT_DIR environment variable override
	if envDir := os.Getenv("CYPHR_AGENT_DIR"); envDir != "" {
		cleanEnvDir := filepath.Clean(envDir)
		if stat, err := os.Stat(cleanEnvDir); err == nil && stat.IsDir() {
			abs, _ := filepath.Abs(cleanEnvDir)
			return abs
		}
	}

	// 2. Try relative to current working directory
	cwd, err := os.Getwd()
	if err == nil {
		candidates := []string{
			filepath.Join(cwd, "backend", "agent"),
			filepath.Join(cwd, "..", "backend", "agent"),
			filepath.Join(cwd, "agent"),
			cwd,
		}
		for _, c := range candidates {
			if IsAgentDir(c) {
				abs, _ := filepath.Abs(c)
				return abs
			}
		}
	}

	// 3. Try relative to executable directory
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "agent"),
			filepath.Join(exeDir, "backend", "agent"),
			filepath.Join(exeDir, "..", "backend", "agent"),
			filepath.Join(exeDir, "..", "..", "backend", "agent"),
			exeDir,
		}
		for _, c := range candidates {
			if IsAgentDir(c) {
				abs, _ := filepath.Abs(c)
				return abs
			}
		}
	}

	// 4. Default target for a standalone deployment: ./agent in current directory
	if cwd != "" {
		return filepath.Join(cwd, "agent")
	}
	return "agent"
}

// IsAgentDir checks whether a directory is an installed agent folder.
func IsAgentDir(dir string) bool {
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		mainPy := filepath.Join(dir, "main.py")
		if _, err := os.Stat(mainPy); err == nil {
			return true
		}
	}
	return false
}

// DetectPython finds the best Python interpreter to use for the agent.
func DetectPython(agentDir string) string {
	candidates := []string{
		filepath.Join(agentDir, ".venv", "Scripts", "python.exe"),
		filepath.Join(agentDir, ".venv", "Scripts", defaultPythonCmd),
		filepath.Join(agentDir, ".venv", "bin", defaultPythonCmd),
		filepath.Join(agentDir, ".venv", "bin", python3Cmd),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}

	if runtime.GOOS == "windows" {
		for _, name := range []string{"python.exe", defaultPythonCmd, "python3.exe", python3Cmd} {
			if path, err := exec.LookPath(name); err == nil {
				if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
					return path
				}
			}
		}
		return defaultPythonCmd
	}

	for _, name := range []string{python3Cmd, defaultPythonCmd} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return python3Cmd
}

// NewAppPathsFromDir initializes AppPaths for a specific agent directory.
func NewAppPathsFromDir(agentDir string) *AppPaths {
	absAgent, _ := filepath.Abs(agentDir)
	rootDir := filepath.Dir(filepath.Dir(absAgent))

	return &AppPaths{
		RootDir:          rootDir,
		AgentDir:         absAgent,
		PidFile:          filepath.Join(absAgent, "agent.pid"),
		LogFile:          filepath.Join(absAgent, "agent.log"),
		ConfigFile:       filepath.Join(absAgent, "config.yaml"),
		ConfigExample:    filepath.Join(absAgent, "config.example.yaml"),
		EnvFile:          filepath.Join(absAgent, ".env"),
		DownloadPidFile:  filepath.Join(absAgent, "download.pid"),
		DownloadLogFile:  filepath.Join(absAgent, "download.log"),
		DownloadInfoFile: filepath.Join(absAgent, "download.info.log"),
		ModelsDir:        filepath.Join(absAgent, "models"),
		PythonBin:        DetectPython(absAgent),
	}
}

// NewAppPaths initializes all paths using the resolved agent directory.
func NewAppPaths() *AppPaths {
	return NewAppPathsFromDir(FindAgentRoot())
}

// LoadEnv loads key-value pairs from .env if it exists.
func LoadEnv(envFile string) map[string]string {
	res := make(map[string]string)
	f, err := os.Open(filepath.Clean(envFile)) //nolint:gosec // Local .env file
	if err != nil {
		return res
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", envKeyValueParts)
		if len(parts) == envKeyValueParts {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			res[k] = v
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
	}
	return res
}
