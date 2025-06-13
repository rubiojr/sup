package store

import (
	"fmt"

	"github.com/rubiojr/kv"
)

type Store interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
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

func (s *kvStore) Namespace(name string) Store {
	return &kvStore{
		db:        s.db,
		namespace: fmt.Sprintf("%s:", name),
	}
}
