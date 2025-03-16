package cachers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// indexEntry is the metadata that DiskCache stores on disk for an ActionID.
type indexEntry struct {
	OutputID  string `json:"o"`
	Size      int64  `json:"n"`
	TimeNanos int64  `json:"t"`
}

type DiskCache struct {
	// Cache input directories.
	// Normally from NIX_GOCACHE
	InputDirs []string

	// Cache output directory path.
	OutDir string

	// Timestamp to store put requests with.
	// Normally derived from SOURCE_DATE_EPOCH.
	TimeNanos int64

	// Debug cache hits/misses
	Verbose bool

	// Wait for in flight writes to finish on shutdown
	wg sync.WaitGroup
}

func (dc *DiskCache) Get(ctx context.Context, actionID string) (outputID, diskPath string, err error) {
	filename := fmt.Sprintf("a-%s", actionID)

	for _, dir := range dc.InputDirs {
		actionFile := filepath.Join(dir, filename)

		ij, err := os.ReadFile(actionFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return "", "", nil
		}

		var ie indexEntry
		if err := json.Unmarshal(ij, &ie); err != nil {
			log.Printf("Warning: JSON error for action %q: %v", actionID, err)
			return "", "", nil
		}

		if _, err := hex.DecodeString(ie.OutputID); err != nil {
			// Protect against malicious non-hex OutputID on disk
			return "", "", nil
		}

		return ie.OutputID, filepath.Join(dir, fmt.Sprintf("o-%v", ie.OutputID)), nil
	}

	if dc.Verbose {
		log.Printf("disk miss: %v", actionID)
	}

	return "", "", nil
}

func (dc *DiskCache) Put(ctx context.Context, actionID, outputID string, size int64, body io.Reader) (diskPath string, _ error) {
	dc.wg.Add(1)
	defer dc.wg.Done()

	if dc.OutDir == "" {
		return "", fmt.Errorf("received put but no output directory was set")
	}

	file := filepath.Join(dc.OutDir, fmt.Sprintf("o-%s", outputID))

	// Special case empty files; they're both common and easier to do race-free.
	if size == 0 {
		zf, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return "", err
		}
		zf.Close()
	} else {
		wrote, err := writeAtomic(file, body)
		if err != nil {
			return "", err
		}
		if wrote != size {
			return "", fmt.Errorf("wrote %d bytes, expected %d", wrote, size)
		}
	}

	ij, err := json.Marshal(indexEntry{
		OutputID:  outputID,
		Size:      size,
		TimeNanos: dc.TimeNanos,
	})
	if err != nil {
		return "", err
	}

	actionFile := filepath.Join(dc.OutDir, fmt.Sprintf("a-%s", actionID))
	if _, err := writeAtomic(actionFile, bytes.NewReader(ij)); err != nil {
		return "", err
	}

	return file, nil
}

func (dc *DiskCache) Wait() {
	dc.wg.Wait()
}

func writeAtomic(dest string, r io.Reader) (int64, error) {
	tf, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*")
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(tf, r)
	if err != nil {
		tf.Close()
		os.Remove(tf.Name())
		return 0, err
	}
	if err := tf.Close(); err != nil {
		os.Remove(tf.Name())
		return 0, err
	}
	if err := os.Rename(tf.Name(), dest); err != nil {
		os.Remove(tf.Name())
		return 0, err
	}
	return size, nil
}
