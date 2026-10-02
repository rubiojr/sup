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
	"go.mau.fi/whatsmeow"
)

var errFileLimit = errors.New("archive file size limit exceeded")
var errMediaLimit = errors.New("archive media storage limit exceeded")

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
	if size, hash, ok := a.recoverDownload(job); ok {
		return a.finishJob(job, "ready", "", size, hash)
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
	case errors.Is(err, errFileLimit):
		state, reason = "skipped", "file_size_limit"
	case errors.Is(err, errMediaLimit):
		state, reason = "skipped", "media_storage_limit"
	case errors.Is(err, context.Canceled):
		reason = "interrupted"
	case job.attempts >= 3:
		state = "failed"
	}
	// Upstream errors can contain signed media URLs. Persist a category instead.
	log.Warn("Archive attachment not downloaded", "attachment", job.key, "reason", reason, "state", state)
	return a.finishJob(job, state, reason, 0, "")
}

func (a *Archive) recoverDownload(job *downloadJob) (int64, string, bool) {
	f, err := a.root.Open(job.path)
	if err != nil {
		return 0, "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > a.cfg.MaxFileBytes {
		return 0, "", false
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, io.LimitReader(f, a.cfg.MaxFileBytes+1)); err != nil {
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
	a.mediaBytes -= info.Size()
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
	remaining := a.cfg.MaxMediaBytes - a.mediaBytes
	if remaining <= 32 {
		return 0, "", errMediaLimit
	}
	limit := min(a.cfg.MaxFileBytes+32, remaining)
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
			// Failed cleanup must stay charged against the media budget.
			info, statErr := a.root.Stat(part)
			if removeErr := a.root.Remove(part); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				if statErr == nil {
					a.mediaBytes += info.Size()
				}
				err = errors.Join(err, removeErr)
			}
		}
	}()
	bounded := &limitedFile{file: f, limit: limit}
	if err := a.downloader.DownloadToFile(ctx, &job.descriptor, bounded); err != nil {
		if errors.Is(err, errFileLimit) && limit < a.cfg.MaxFileBytes+32 {
			return 0, "", errMediaLimit
		}
		return 0, "", err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if info.Size() > a.cfg.MaxFileBytes {
		return 0, "", errFileLimit
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
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
	a.mediaBytes += info.Size()
	return info.Size(), hex.EncodeToString(hasher.Sum(nil)), nil
}

// Do not embed *os.File: promoted ReadFrom methods would bypass Write's bound.
type limitedFile struct {
	file  *os.File
	limit int64
}

var _ whatsmeow.File = (*limitedFile)(nil)

func (f *limitedFile) Read(p []byte) (int, error)              { return f.file.Read(p) }
func (f *limitedFile) ReadAt(p []byte, off int64) (int, error) { return f.file.ReadAt(p, off) }
func (f *limitedFile) Stat() (os.FileInfo, error)              { return f.file.Stat() }

func (f *limitedFile) Write(p []byte) (int, error) {
	off, err := f.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	if off < 0 || off > f.limit || int64(len(p)) > f.limit-off {
		return 0, errFileLimit
	}
	return f.file.Write(p)
}

func (f *limitedFile) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off > f.limit || int64(len(p)) > f.limit-off {
		return 0, errFileLimit
	}
	return f.file.WriteAt(p, off)
}

func (f *limitedFile) Truncate(size int64) error {
	if size < 0 || size > f.limit {
		return errFileLimit
	}
	return f.file.Truncate(size)
}

func (f *limitedFile) Seek(off int64, whence int) (int64, error) {
	// Reads may seek beyond EOF; writes and truncation enforce the actual bound.
	return f.file.Seek(off, whence)
}
