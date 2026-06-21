package browse

import (
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

func TestParseQuery(t *testing.T) {
	query := ParseQuery("quiet tag:demo actress:alice other")

	if query.Text != "quiet other" {
		t.Fatalf("Text = %q", query.Text)
	}
	if query.Tag != "demo" {
		t.Fatalf("Tag = %q", query.Tag)
	}
	if query.Performer != "alice" {
		t.Fatalf("Performer = %q", query.Performer)
	}
}

func TestCollectPrimaryFileIDs(t *testing.T) {
	primary := models.FileID(2)
	got := collectPrimaryFileIDs([]*models.Scene{
		{ID: 1, PrimaryFileID: &primary},
		{ID: 2},
		{ID: 3, PrimaryFileID: &primary},
	}, [][]models.FileID{
		{1, 2},
		{3, 4},
		{2},
	})
	want := []models.FileID{2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectPrimaryFileIDs = %#v, want %#v", got, want)
	}
}
