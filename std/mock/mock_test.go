package mock

import (
	"errors"
	"testing"
)

func TestMockStore(t *testing.T) {
	wantErr := errors.New("save failed")
	store := MockStore{
		GetFunc:  func(id string) (User, error) { return User{ID: id}, nil },
		SaveFunc: func(User) error { return wantErr },
	}

	user, err := store.Get("42")
	if err != nil || user.ID != "42" {
		t.Fatalf("Get() = (%+v, %v), want user 42 and nil", user, err)
	}
	if err := store.Save(user); !errors.Is(err, wantErr) {
		t.Errorf("Save() error = %v, want %v", err, wantErr)
	}
}
