package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	"k8s.io/klog/v2"
)

// klog log file name pattern, e.g., "program.hostname.user.log.severity.timestamp.pid"
// We are interested in the timestamp part.
// Example: KubeMin-Cli.localhost.kai.log.INFO.20250725-160435.27455
var klogLogNamePattern = regexp.MustCompile(`.+\..+\..+\.log\.(INFO|WARNING|ERROR|FATAL)\.(\d{8}-\d{6})\.\d+$`)

const (
	klogTimestampFormat  = "20060102-150405"
	cleanupCheckInterval = 24 * time.Hour
	defaultMaxLogAge     = 7 * 24 * time.Hour // 7 days
)

// StartLogCleanup starts a goroutine that periodically cleans up old log files in the specified directory.
func StartLogCleanup(logDir string, maxAge time.Duration) {
	if logDir == "" {
		klog.Warningf("Log cleanup is disabled because log_dir is not set.")
		return
	}
	if maxAge <= 0 {
		maxAge = defaultMaxLogAge
		klog.Warningf("Invalid max log age provided, defaulting to %v", maxAge)
	}

	klog.Infof("Starting log cleanup service for directory %s, with max age %v", logDir, maxAge)

	go func() {
		// Run cleanup immediately on start, then tick every 24 hours.
		cleanup(logDir, maxAge)
		ticker := time.NewTicker(cleanupCheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			cleanup(logDir, maxAge)
		}
	}()
}

func cleanup(logDir string, maxAge time.Duration) {
	klog.V(4).Infof("Running log cleanup in directory: %s", logDir)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		klog.Errorf("Failed to read log directory %s for cleanup: %v", logDir, err)
		return
	}

	cutoffTime := time.Now().Add(-maxAge)
	filesDeleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		matches := klogLogNamePattern.FindStringSubmatch(fileName)
		// matches[0] is the full string, matches[1] is severity, matches[2] is timestamp
		if len(matches) < 3 {
			klog.V(5).Infof("Skipping file with non-matching name: %s", fileName)
			continue
		}

		timestampStr := matches[2]
		logTime, err := time.Parse(klogTimestampFormat, timestampStr)
		if err != nil {
			klog.Warningf("Could not parse timestamp from log file name %s: %v", fileName, err)
			continue
		}

		if logTime.Before(cutoffTime) {
			filePath := filepath.Join(logDir, fileName)
			klog.Infof("Deleting old log file: %s", filePath)
			if err := os.Remove(filePath); err != nil {
				klog.Errorf("Failed to delete old log file %s: %v", filePath, err)
			} else {
				filesDeleted++
			}
		}
	}
	klog.V(2).Infof("Log cleanup finished. Deleted %d file(s).", filesDeleted)
}
