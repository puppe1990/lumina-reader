package main

import (
	"log"

	"github.com/puppe1990/amarra-cais/pkg/cais"
	"github.com/puppe1990/amarra-cais/pkg/cais/console"

	"github.com/puppe1990/lumina/internal/store"
)

func openStore(cfg cais.Config) (*store.SQLiteStore, error) {
	return store.NewSQLiteStore(cfg.DBPath, cfg.Env)
}

func bindings(s *store.SQLiteStore) map[string]any {
	return map[string]any{
		"store": s,
		"db":    s.DB(),
	}
}

func main() {
	if err := run(cais.Load()); err != nil {
		log.Fatal(err)
	}
}

func run(cfg cais.Config) error {
	s, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	active := s
	return console.Run(console.Options{
		AppName:  "lumina",
		Config:   cfg,
		Bindings: bindings(active),
		Reload: func() (map[string]any, error) {
			_ = active.Close()
			next, err := openStore(cfg)
			if err != nil {
				return nil, err
			}
			active = next
			return bindings(active), nil
		},
	})
}
