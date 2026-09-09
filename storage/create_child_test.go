package storage_test

import (
	"testing"

	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"

	_ "github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// childCreatingRepo stands in for a store that names children itself — Android
// SAF, where the provider assigns the document ID. Child() cannot answer for a
// name that is not there, so CreateChild is the only way in.
type childCreatingRepo struct {
	*intRepo.InMemoryRepository

	created []string
}

func (r *childCreatingRepo) Child(u bellui.URI, name string) (bellui.URI, error) {
	child, err := r.InMemoryRepository.Child(u, name)
	if err != nil {
		return nil, err
	}
	if exists, _ := r.Exists(child); !exists {
		return nil, storage.ErrNotExists
	}
	return child, nil
}

func (r *childCreatingRepo) CreateChild(parent bellui.URI, name string, listable bool) (bellui.URI, error) {
	child, err := r.InMemoryRepository.Child(parent, name)
	if err != nil {
		return nil, err
	}
	r.created = append(r.created, child.String())
	if listable {
		return child, r.CreateListable(child)
	}
	r.Data[child.Path()] = []byte{}
	return child, nil
}

func TestCreateChild_delegatesToTheRepository(t *testing.T) {
	m := &childCreatingRepo{InMemoryRepository: intRepo.NewInMemoryRepository("createchild")}
	repository.Register("createchild", m)
	require.NoError(t, m.CreateListable(mustURI(t, "createchild:///")))

	root := mustURI(t, "createchild:///")

	// Child() cannot name what is not there — the state that makes
	// CreateChild necessary in the first place.
	_, err := storage.Child(root, "project")
	assert.ErrorIs(t, err, storage.ErrNotExists)

	dir, err := storage.CreateChild(root, "project", true)
	require.NoError(t, err)
	assert.Equal(t, "createchild:///project", dir.String())
	assert.Equal(t, []string{"createchild:///project"}, m.created)

	// Now that it exists, Child() finds it.
	found, err := storage.Child(root, "project")
	require.NoError(t, err)
	assert.Equal(t, dir.String(), found.String())

	file, err := storage.CreateChild(dir, "main.go", false)
	require.NoError(t, err)
	assert.Equal(t, "createchild:///project/main.go", file.String())

	exists, err := storage.Exists(file)
	require.NoError(t, err)
	assert.True(t, exists, "a non-listable child should exist once created")
}

func TestCreateChild_fallsBackToChildAndCreate(t *testing.T) {
	// The file repository is the subject that matters for the fallback: it is
	// what CreateChild runs against on every desktop target when no repository
	// claims the capability.
	root := storage.NewFileURI(t.TempDir())

	dir, err := storage.CreateChild(root, "project", true)
	require.NoError(t, err)

	listable, err := storage.CanList(dir)
	require.NoError(t, err)
	assert.True(t, listable, "a listable child should list")

	file, err := storage.CreateChild(dir, "main.go", false)
	require.NoError(t, err)

	exists, err := storage.Exists(file)
	require.NoError(t, err)
	assert.True(t, exists, "the fallback must create the resource, not just derive its URI")

	r, err := storage.Reader(file)
	require.NoError(t, err)
	require.NoError(t, r.Close())
}

// TestCreateChild_fallbackReportsAnUncreatedChild pins the check that turns a
// repository whose Writer stores nothing until written to — the in-memory one —
// into an error rather than a URI pointing at nothing.
func TestCreateChild_fallbackReportsAnUncreatedChild(t *testing.T) {
	m := intRepo.NewInMemoryRepository("createchildlazy")
	repository.Register("createchildlazy", m)
	require.NoError(t, m.CreateListable(mustURI(t, "createchildlazy:///")))

	_, err := storage.CreateChild(mustURI(t, "createchildlazy:///"), "main.go", false)
	assert.ErrorContains(t, err, "did not create")
}

func TestCreateChild_unsupportedScheme(t *testing.T) {
	u := mustURI(t, "nosuchscheme:///foo")
	_, err := storage.CreateChild(u, "bar", true)
	assert.Error(t, err)
}

func mustURI(t *testing.T, s string) bellui.URI {
	t.Helper()
	u, err := storage.ParseURI(s)
	require.NoError(t, err)
	return u
}
