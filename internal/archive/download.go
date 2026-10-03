package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rubiojr/sup/internal/log"
)

// Run processes the durable download queue with one worker. Cancellation leaves
// pending jobs on disk and interrupts the current download. It never sends replies.
func (a *Archive) Run(ctx context.Context) error {
	if !a.running.CompareAndSwap(false, true) {
		return errors.New("archive worker is already running")
	}
	defer a.running.Store(false)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		job, err := a.nextJob(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("reading archive queue: %w", err)
		}
		if job != nil {
			if err := a.processJob(ctx, job); err != nil {
				return err
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-a.wake:
		case <-ticker.C:
		}
	}
}

func (a *Archive) processJob(ctx context.Context, job *downloadJob) error {
	// A crash may leave a complete file between the atomic rename and DB update.
	if size, hash, ok := a.recoverDownload(ctx, job); ok {
		return a.finishJob(job, "ready", "", size, hash)
	}
	if ctx.Err() != nil {
		return a.finishJob(job, "retry", "interrupted", 0, "")
	}
	if job.attempts > 3 {
		if err := a.removePartial(job.path + ".part"); err != nil {
			return err
		}
		return a.finishJob(job, "failed", "attempt_limit", 0, "")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	size, hash, err := a.download(ctx, job)
	if err == nil {
		return a.finishJob(job, "ready", "", size, hash)
	}
	state, reason := "retry", "download_failed"
	switch {
	case errors.Is(err, context.Canceled):
		reason = "interrupted"
	case job.attempts >= 3:
		state = "failed"
	}
	// Upstream errors can contain signed media URLs. Persist a category instead.
	log.Warn("Archive attachment not downloaded", "attachment", job.key, "reason", reason, "state", state)
	return a.finishJob(job, state, reason, 0, "")
}

func (a *Archive) recoverDownload(ctx context.Context, job *downloadJob) (int64, string, bool) {
	f, err := a.root.Open(job.path)
	if err != nil {
		return 0, "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return 0, "", false
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, contextReader{ctx: ctx, reader: f}); err != nil {
		return 0, "", false
	}
	hash := hasher.Sum(nil)
	if !bytes.Equal(hash, job.descriptor.FileHash) {
		return 0, "", false
	}
	return info.Size(), hex.EncodeToString(hash), true
}

func (a *Archive) removePartial(path string) error {
	info, err := a.root.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("archive media path is not a regular file")
	}
	if err := a.root.Remove(path); err != nil {
		return err
	}
	return nil
}

func (a *Archive) download(ctx context.Context, job *downloadJob) (size int64, hash string, err error) {
	// These names derive solely from hashed IDs. os.Root also confines symlinks.
	part := job.path + ".part"
	for _, path := range []string{part, job.path} {
		if err := a.removePartial(path); err != nil {
			return 0, "", err
		}
	}
	if err := a.root.MkdirAll(filepath.Dir(job.path), 0o700); err != nil {
		return 0, "", err
	}
	f, err := a.root.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		_ = f.Close()
		if err != nil {
			if removeErr := a.root.Remove(part); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if err := a.downloader.DownloadToFile(ctx, &job.descriptor, f); err != nil {
		return 0, "", err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, contextReader{ctx: ctx, reader: f}); err != nil {
		return 0, "", err
	}
	if err := f.Sync(); err != nil {
		return 0, "", err
	}
	if err := f.Close(); err != nil {
		return 0, "", err
	}
	if err := a.root.Rename(part, job.path); err != nil {
		return 0, "", err
	}
	return info.Size(), hex.EncodeToString(hasher.Sum(nil)), nil
}

// Hashing large files uses constant memory and can be interrupted on shutdown.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
