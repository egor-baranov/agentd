package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Metadata struct {
	Registry   RegistryStore
	Bundles    BundleStore
	Containers ContainerStore
	Sessions   SessionStore
	Artifacts  ArtifactStore
	Close      func() error
}

func Open(ctx context.Context, databaseURL, artifactDir string) (*Metadata, error) {
	if databaseURL == "" {
		mem := NewMemoryStore()
		artifacts := ArtifactStore(mem)
		if artifactDir != "" {
			artifacts = NewFSArtifactStore(artifactDir)
		}
		return &Metadata{Registry: mem, Bundles: mem, Containers: mem, Sessions: mem, Artifacts: artifacts, Close: func() error { return nil }}, nil
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	pg := NewPostgresStore(pool)
	if err := pg.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	artifacts := ArtifactStore(NewFSArtifactStore(artifactDir))
	return &Metadata{Registry: pg, Bundles: pg, Containers: pg, Sessions: pg, Artifacts: artifacts, Close: func() error { pool.Close(); return nil }}, nil
}
