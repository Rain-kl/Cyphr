package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// AppPaths holds all relevant filesystem paths for the agent and installer.
type AppPaths struct {
	RootDir          string
	AgentDir         string
	PidFile          string
	LogFile          string
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
		if stat, err := os.Stat(envDir); err == nil && stat.IsDir() {
			abs, _ := filepath.Abs(envDir)
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
			if isAgentDir(c) {
				abs, _ := filepath.Abs(c)
				return abs
			}
		}
	}

	// 3. Try relative to executable directory
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "backend", "agent"),
			filepath.Join(exeDir, "..", "backend", "agent"),
			filepath.Join(exeDir, "..", "..", "backend", "agent"),
		}
		for _, c := range candidates {
			if isAgentDir(c) {
				abs, _ := filepath.Abs(c)
				return abs
			}
		}
	}

	// 4. Default fallback to standard repository path
	return "/Users/ryan/Code/Go/Cyphr/backend/agent"
}

func isAgentDir(dir string) bool {
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
	venvPython := filepath.Join(agentDir, ".venv", "bin", "python")
	if fi, err := os.Stat(venvPython); err == nil && !fi.IsDir() {
		return venvPython
	}
	return "python3"
}

// NewAppPaths initializes all paths using the resolved agent directory.
func NewAppPaths() *AppPaths {
	agentDir := FindAgentRoot()
	rootDir := filepath.Dir(filepath.Dir(agentDir)) // Cyphr repo root

	return &AppPaths{
		RootDir:          rootDir,
		AgentDir:         agentDir,
		PidFile:          filepath.Join(agentDir, "agent.pid"),
		LogFile:          filepath.Join(agentDir, "agent.log"),
		EnvFile:          filepath.Join(agentDir, ".env"),
		DownloadPidFile:  filepath.Join(agentDir, "download.pid"),
		DownloadLogFile:  filepath.Join(agentDir, "download.log"),
		DownloadInfoFile: filepath.Join(agentDir, "download.info.log"),
		ModelsDir:        filepath.Join(agentDir, "models"),
		PythonBin:        DetectPython(agentDir),
	}
}

// LoadEnv loads key-value pairs from .env if it exists.
func LoadEnv(envFile string) map[string]string {
	res := make(map[string]string)
	f, err := os.Open(envFile)
	if err != nil {
		return res
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			res[k] = v
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
	return res
}
