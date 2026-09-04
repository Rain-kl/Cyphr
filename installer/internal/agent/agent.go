package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cyphr/installer/internal/config"
	"cyphr/installer/internal/proc"
)

// AgentStatus represents current agent runtime status.
type AgentStatus struct {
	Running bool
	PID     int
	Uptime  string
	RSSMB   int
	LogPath string
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

// Status inspects the current agent daemon status.
func (s *Service) Status() *AgentStatus {
	pid, err := proc.ReadPid(s.paths.PidFile)
	if err != nil || pid <= 0 || !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.PidFile)
		return &AgentStatus{
			Running:    false,
			LogPath:    s.paths.LogFile,
			RecentLogs: proc.TailLines(s.paths.LogFile, 8),
		}
	}

	stats := proc.GetStats(pid)
	return &AgentStatus{
		Running:    true,
		PID:        pid,
		Uptime:     stats.Uptime,
		RSSMB:      stats.RSSMB,
		LogPath:    s.paths.LogFile,
		RecentLogs: proc.TailLines(s.paths.LogFile, 8),
	}
}

// Start launches the agent daemon in background.
func (s *Service) Start() (*AgentStatus, error) {
	st := s.Status()
	if st.Running {
		return st, fmt.Errorf("agent service is already running (PID: %d)", st.PID)
	}

	proc.RemovePid(s.paths.PidFile)

	pythonBin := s.paths.PythonBin
	if pythonBin == "" {
		return nil, fmt.Errorf("python interpreter not found")
	}

	mainPy := filepath.Join(s.paths.AgentDir, "main.py")
	if _, err := os.Stat(mainPy); err != nil {
		return nil, fmt.Errorf("agent entrypoint not found: %s", mainPy)
	}

	command := []string{pythonBin, mainPy}
	// Load .env variables
	_ = config.LoadEnv(s.paths.EnvFile)

	pid, err := proc.Daemonize(command, nil, s.paths.AgentDir, s.paths.LogFile)
	if err != nil {
		return nil, fmt.Errorf("daemonize agent failed: %w", err)
	}

	if err := proc.WritePid(s.paths.PidFile, pid); err != nil {
		return nil, fmt.Errorf("save pid failed: %w", err)
	}

	// Wait 1.2s to confirm process survived initialization
	time.Sleep(1200 * time.Millisecond)

	if !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.PidFile)
		recent := proc.TailLines(s.paths.LogFile, 10)
		return nil, fmt.Errorf("agent exited unexpectedly shortly after start. Latest logs:\n%v", recent)
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

	err := proc.GracefulStop(st.PID, 5*time.Second)
	proc.RemovePid(s.paths.PidFile)
	return err
}

// Restart restarts the agent service.
func (s *Service) Restart() (*AgentStatus, error) {
	_ = s.Stop()
	time.Sleep(500 * time.Millisecond)
	return s.Start()
}
