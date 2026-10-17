package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func getServicePaths() (pidPath string, logPath string) {
	home, err := os.UserHomeDir()
	if err == nil {
		testFile := filepath.Join(home, ".gatorservice.test")
		if testErr := os.WriteFile(testFile, []byte(""), 0644); testErr == nil {
			_ = os.Remove(testFile)
			return filepath.Join(home, ".gatorservice.pid"), filepath.Join(home, ".gatorservice.log")
		}
	}
	return ".gatorservice.pid", ".gatorservice.log"
}

func readPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	return strconv.Atoi(pidStr)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func handlerService(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		printServiceUsage()
		return nil
	}

	subcommand := cmd.args[0]
	pidFile, logFile := getServicePaths()

	switch subcommand {
	case "start":
		return serviceStart(cmd.args[1:], pidFile, logFile)
	case "status":
		return serviceStatus(pidFile, logFile)
	case "stop":
		return serviceStop(pidFile)
	case "logs":
		follow := len(cmd.args) > 1 && cmd.args[1] == "-f"
		return serviceLogs(logFile, follow)
	case "run-supervisor":
		return serviceRunSupervisor(cmd.args[1:], logFile)
	default:
		printServiceUsage()
		return fmt.Errorf("unknown service subcommand: %s", subcommand)
	}
}

func printServiceUsage() {
	fmt.Println("Usage: gator service <command> [args...]")
	fmt.Println("Commands:")
	fmt.Println("  start [duration] [workers]  - Start the aggregator service in the background")
	fmt.Println("  status                     - Show aggregator service health and PID")
	fmt.Println("  stop                       - Stop the background aggregator service")
	fmt.Println("  logs [-f]                  - View or follow service logs")
}

func serviceStart(args []string, pidFile, logFile string) error {
	if pid, err := readPID(pidFile); err == nil && isProcessAlive(pid) {
		return fmt.Errorf("service is already running (PID: %d)", pid)
	}

	duration := "1m"
	if len(args) > 0 {
		duration = args[0]
	}
	workers := "3"
	if len(args) > 1 {
		workers = args[1]
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare supervisor command
	supervisorCmd := exec.Command(execPath, "service", "run-supervisor", duration, workers)
	
	logOut, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logFile, err)
	}
	defer logOut.Close()

	supervisorCmd.Stdout = logOut
	supervisorCmd.Stderr = logOut

	if err := supervisorCmd.Start(); err != nil {
		return fmt.Errorf("failed to start supervisor: %w", err)
	}

	pid := supervisorCmd.Process.Pid
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	fmt.Printf("Gator aggregator service started successfully!\n")
	fmt.Printf("  PID:      %d\n", pid)
	fmt.Printf("  Interval: %s\n", duration)
	fmt.Printf("  Workers:  %s\n", workers)
	fmt.Printf("  Logs:     %s\n", logFile)
	return nil
}

func serviceStatus(pidFile, logFile string) error {
	pid, err := readPID(pidFile)
	if err != nil || !isProcessAlive(pid) {
		fmt.Println("Gator service status: STOPPED")
		return nil
	}

	fmt.Printf("Gator service status: RUNNING (PID: %d)\n", pid)
	fmt.Printf("Logs: %s\n\n", logFile)

	// Display recent log lines
	if data, err := os.ReadFile(logFile); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		start := len(lines) - 10
		if start < 0 {
			start = 0
		}
		fmt.Println("--- Recent Logs ---")
		for _, line := range lines[start:] {
			fmt.Println(line)
		}
	}
	return nil
}

func serviceStop(pidFile string) error {
	pid, err := readPID(pidFile)
	if err != nil || !isProcessAlive(pid) {
		fmt.Println("Gator service is not running.")
		_ = os.Remove(pidFile)
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	_ = process.Signal(syscall.SIGTERM)

	// Wait for process to exit
	stopped := false
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		if !isProcessAlive(pid) {
			stopped = true
			break
		}
	}

	if !stopped {
		_ = process.Signal(syscall.SIGKILL)
	}

	_ = os.Remove(pidFile)
	fmt.Printf("Gator service (PID: %d) stopped.\n", pid)
	return nil
}

func serviceLogs(logFile string, follow bool) error {
	if !follow {
		data, err := os.ReadFile(logFile)
		if err != nil {
			return fmt.Errorf("could not read log file: %w", err)
		}
		fmt.Print(string(data))
		return nil
	}

	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("could not open log file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		fmt.Print(line)
	}
}

func serviceRunSupervisor(args []string, logFile string) error {
	duration := "1m"
	if len(args) > 0 {
		duration = args[0]
	}
	workers := "3"
	if len(args) > 1 {
		workers = args[1]
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	log.Printf("[SUPERVISOR] Started aggregator supervisor (Interval: %s, Workers: %s)\n", duration, workers)

	for {
		startTime := time.Now()
		cmd := exec.Command(execPath, "agg", duration, workers)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			log.Printf("[SUPERVISOR] Failed to spawn agg process: %v\n", err)
			time.Sleep(backoff)
			continue
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case sig := <-sigChan:
			log.Printf("[SUPERVISOR] Received signal %v. Terminating aggregator...\n", sig)
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			return nil

		case exitErr := <-done:
			if time.Since(startTime) > 60*time.Second {
				backoff = 1 * time.Second
			}
			log.Printf("[SUPERVISOR] Aggregator exited (code: %v). Restarting in %v...\n", exitErr, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
