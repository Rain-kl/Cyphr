// Package agent manages the lifecycle, installation, and status of the Cyphr agent.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cyphr/installer/internal/config"
	"cyphr/installer/internal/proc"
)

const (
	recentLogLines = 8
	errorLogLines  = 10

	agentStartupDelay = 1200 * time.Millisecond
	agentStopTimeout  = 5 * time.Second
	restartDelay      = 500 * time.Millisecond
)

// Status represents current agent runtime status.
type Status struct {
	Installed  bool
	Running    bool
	PID        int
	Uptime     string
	RSSMB      int
	AgentDir   string
	LogPath    string
	RecentLogs []string
}

// Service manages the agent lifecycle.
type Service struct {
	paths *config.AppPaths
}

// NewService creates a new Service instance.
func NewService(paths *config.AppPaths) *Service {
	return &Service{paths: paths}
}

// Paths returns current paths configuration.
func (s *Service) Paths() *config.AppPaths {
	return s.paths
}

// IsInstalled checks if agent entrypoint exists.
func (s *Service) IsInstalled() bool {
	mainPy := filepath.Join(s.paths.AgentDir, "main.py")
	_, err := os.Stat(mainPy)
	return err == nil
}

// Status inspects the current agent daemon status.
func (s *Service) Status() *Status {
	installed := s.IsInstalled()

	pid, err := proc.ReadPid(s.paths.PidFile)
	if err != nil || pid <= 0 || !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.PidFile)
		return &Status{
			Installed:  installed,
			Running:    false,
			AgentDir:   s.paths.AgentDir,
			LogPath:    s.paths.LogFile,
			RecentLogs: proc.TailLines(s.paths.LogFile, recentLogLines),
		}
	}

	stats := proc.GetStats(pid)
	return &Status{
		Installed:  installed,
		Running:    true,
		PID:        pid,
		Uptime:     stats.Uptime,
		RSSMB:      stats.RSSMB,
		AgentDir:   s.paths.AgentDir,
		LogPath:    s.paths.LogFile,
		RecentLogs: proc.TailLines(s.paths.LogFile, recentLogLines),
	}
}

// Start launches the agent daemon in background.
func (s *Service) Start() (*Status, error) {
	if !s.IsInstalled() {
		return nil, fmt.Errorf("agent 尚未安装 (未找到 %s/main.py)，请先选择【安装/更新 Agent】", s.paths.AgentDir)
	}

	st := s.Status()
	if st.Running {
		return st, fmt.Errorf("agent 服务已在运行中 (PID: %d)", st.PID)
	}

	proc.RemovePid(s.paths.PidFile)

	pythonBin := s.paths.PythonBin
	if pythonBin == "" {
		return nil, fmt.Errorf("未找到 Python 解释器，请先安装 Python >= 3.12")
	}

	mainPy := filepath.Join(s.paths.AgentDir, "main.py")
	command := []string{pythonBin, "-u", mainPy}
	// Load .env variables
	_ = config.LoadEnv(s.paths.EnvFile)
	agentEnv := []string{
		"PYTHONUNBUFFERED=1",
		"PYTHONPATH=" + s.paths.AgentDir,
	}

	pid, err := proc.Daemonize(command, agentEnv, s.paths.AgentDir, s.paths.LogFile)
	if err != nil {
		return nil, fmt.Errorf("daemonize agent failed: %w", err)
	}

	if err := proc.WritePid(s.paths.PidFile, pid); err != nil {
		return nil, fmt.Errorf("save pid failed: %w", err)
	}

	// Wait to confirm process survived initialization
	time.Sleep(agentStartupDelay)

	if !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.PidFile)
		recent := proc.TailLines(s.paths.LogFile, errorLogLines)
		return nil, fmt.Errorf("agent 服务启动后异常退出，最新日志:\n%v", recent)
	}

	return s.Status(), nil
}

// Stop gracefully stops the running agent.
func (s *Service) Stop() error {
	st := s.Status()
	if !st.Running {
		proc.RemovePid(s.paths.PidFile)
		return nil
	}

	err := proc.GracefulStop(st.PID, agentStopTimeout)
	proc.RemovePid(s.paths.PidFile)
	return err
}

// Restart restarts the agent service.
func (s *Service) Restart() (*Status, error) {
	_ = s.Stop()
	time.Sleep(restartDelay)
	return s.Start()
}
