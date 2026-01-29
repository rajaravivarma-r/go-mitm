package replay

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteRepositorySetGet(t *testing.T) {
	repo, err := NewSQLiteRepository(":memory:", 2*time.Second)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	value := StoredResponse{StatusCode: 204}
	if err := repo.Set(context.Background(), "key", value, false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, found, err := repo.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if got.StatusCode != value.StatusCode {
		t.Fatalf("unexpected status: %d", got.StatusCode)
	}
}

func TestSQLiteRepositoryEmptyPath(t *testing.T) {
	if _, err := NewSQLiteRepository("", time.Second); err == nil {
		t.Fatal("expected error for empty path")
	}
}
