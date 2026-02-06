package store

import (
	"fmt"

	"github.com/rubiojr/kv"
)

type Store interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	List(prefix string) ([]string, error)
	Namespace(name string) Store
}

type kvStore struct {
	db        kv.Database
	namespace string
}

func NewStore(path string) (*kvStore, error) {
	db, err := kv.New("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &kvStore{db: db}
	return store, nil
}

func (s *kvStore) Get(key []byte) ([]byte, error) {
	return s.db.Get(s.namespace + string(key))
}

func (s *kvStore) Put(key []byte, value []byte) error {
	return s.db.Set(s.namespace+string(key), value, nil)
}

func (s *kvStore) Delete(key []byte) error {
	return s.db.Del(s.namespace + string(key))
}

func (s *kvStore) List(prefix string) ([]string, error) {
	fullPrefix := s.namespace + prefix
	rows, err := s.db.Raw().Query("SELECT key FROM key_values WHERE key LIKE ?", fullPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		// Strip the namespace prefix
		if len(s.namespace) > 0 {
			key = key[len(s.namespace):]
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *kvStore) Namespace(name string) Store {
	return &kvStore{
		db:        s.db,
		namespace: fmt.Sprintf("%s:", name),
	}
}
