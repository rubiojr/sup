package archive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/rubiojr/sup/internal/botfs"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Downloader deliberately exposes no sending, plugin, or command capabilities.
type Downloader interface {
	DownloadToFile(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error
}

// Archive owns its database and media directory. Run must finish before Close.
type Archive struct {
	cfg        Config
	db         *sql.DB
	root       *os.Root
	lockDB     *sql.DB
	lockTx     *sql.Tx
	lockCancel context.CancelFunc
	downloader Downloader
	wake       chan struct{}
	running    atomic.Bool
	mu         sync.RWMutex
	closed     bool
	mediaBytes int64 // Owned by the single download worker.
	now        func() time.Time
}

// Open initializes an archive without starting workers or connecting to WhatsApp.
// A separate SQLite write lock prevents concurrent processes from exceeding quotas.
func Open(ctx context.Context, cfg Config, downloader Downloader) (_ *Archive, err error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if downloader == nil {
		return nil, errors.New("archive downloader is required")
	}
	if cfg.Dir == "" {
		cfg.Dir = filepath.Join(botfs.DataDir(), "archive")
	}
	cfg.Dir, err = filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating archive directory: %w", err)
	}
	root, err := os.OpenRoot(cfg.Dir)
	if err != nil {
		return nil, err
	}
	a := &Archive{cfg: cfg, root: root, downloader: downloader, wake: make(chan struct{}, 1), now: time.Now}
	defer func() {
		if err != nil {
			err = errors.Join(err, a.Close())
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	a.lockDB, err = a.openDB("writer-lock.db")
	if err != nil {
		return nil, err
	}
	// The lock transaction must outlive the startup timeout and remain held
	// through shutdown cleanup. Limit acquisition, then detach that timeout.
	lockCtx, lockCancel := context.WithCancel(context.Background())
	a.lockCancel = lockCancel
	stopLockTimeout := context.AfterFunc(ctx, lockCancel)
	a.lockTx, err = a.lockDB.BeginTx(lockCtx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if !stopLockTimeout() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("locking archive (is another bot using it?): %w", err)
	}
	a.db, err = a.openDB("archive.db")
	if err != nil {
		return nil, err
	}
	if err = a.initSchema(ctx); err != nil {
		return nil, err
	}
	if err = root.MkdirAll("chats", 0o700); err != nil {
		return nil, err
	}
	err = fs.WalkDir(root.FS(), "chats", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive contains a non-regular file: %s", path)
		}
		a.mediaBytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("measuring archive media: %w", err)
	}
	return a, nil
}

func (a *Archive) openDB(name string) (*sql.DB, error) {
	// Create private files through os.Root before SQLite opens them by name.
	f, err := a.root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(a.root.Name(), name)}
	pragmas := []string{"busy_timeout(5000)", "foreign_keys(1)"}
	if name == "archive.db" {
		// Apply the cap to replacement connections as well as the first one.
		pragmas = append(pragmas, "max_page_count("+strconv.FormatInt(a.cfg.MaxDBBytes/4096, 10)+")")
	}
	u.RawQuery = url.Values{"_pragma": pragmas}.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Close releases files and the single-writer lock. It is safe to call repeatedly.
func (a *Archive) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	var errs []error
	if a.db != nil {
		errs = append(errs, a.db.Close())
	}
	if a.lockTx != nil {
		if err := a.lockTx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			errs = append(errs, err)
		}
	}
	if a.lockCancel != nil {
		a.lockCancel()
	}
	if a.lockDB != nil {
		errs = append(errs, a.lockDB.Close())
	}
	if a.root != nil {
		errs = append(errs, a.root.Close())
	}
	return errors.Join(errs...)
}

func (a *Archive) accepts(chat types.JID) bool {
	if chat.User == "" {
		return false
	}
	switch chat.Server {
	case types.GroupServer:
		return a.cfg.Scope != "direct"
	case types.DefaultUserServer, types.HiddenUserServer:
		return a.cfg.Scope != "groups"
	default:
		return false
	}
}

// SetChatNames seeds known contact and group names without granting bot access.
func (a *Archive) SetChatNames(ctx context.Context, names map[types.JID]string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return errors.New("archive is closed")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for jid, name := range names {
		if !a.accepts(jid) || name == "" || len(name) > 1024 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, name, is_group) VALUES (?, ?, ?)
			ON CONFLICT(jid) DO UPDATE SET name = excluded.name`, jid.ToNonAD().String(), name, jid.Server == types.GroupServer); err != nil {
			return err
		}
	}
	return tx.Commit()
}
