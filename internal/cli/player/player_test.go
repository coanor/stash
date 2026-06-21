package player

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/cli/browse"
)

type fakeRunner struct {
	path string
	args []string
	err  error
	pos  *float64
}

func (f *fakeRunner) Run(_ context.Context, path string, args []string, progress func(float64)) error {
	f.path = path
	f.args = append([]string(nil), args...)
	if f.pos != nil && progress != nil {
		progress(*f.pos)
	}
	return f.err
}

type fakeRecorder struct {
	sceneID      int
	called       int
	activityID   int
	resumeTime   *float64
	playDuration *float64
}

func (f *fakeRecorder) AddView(_ context.Context, sceneID int, _ time.Time) error {
	f.sceneID = sceneID
	f.called++
	return nil
}

func (f *fakeRecorder) SaveActivity(_ context.Context, sceneID int, resumeTime *float64, playDuration *float64) error {
	f.activityID = sceneID
	f.resumeTime = resumeTime
	f.playDuration = playDuration
	return nil
}

func TestServiceRunsFFplayAndRecordsView(t *testing.T) {
	runner := &fakeRunner{}
	recorder := &fakeRecorder{}
	service := NewWithDeps("/usr/bin/ffplay", []string{"-autoexit"}, runner, recorder)

	err := service.Play(context.Background(), browse.SceneItem{
		ID:    42,
		Title: "Scene",
		Path:  "/tmp/scene.mp4",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if runner.path != "/usr/bin/ffplay" {
		t.Fatalf("runner path = %q, want ffplay path", runner.path)
	}
	if want := []string{"-autoexit", "/tmp/scene.mp4"}; !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("runner args = %#v, want %#v", runner.args, want)
	}
	if recorder.called != 1 || recorder.sceneID != 42 {
		t.Fatalf("recorder called=%d sceneID=%d, want one view for scene 42", recorder.called, recorder.sceneID)
	}
}

func TestServiceDoesNotRecordViewWhenFFplayFails(t *testing.T) {
	runner := &fakeRunner{err: errors.New("ffplay failed")}
	recorder := &fakeRecorder{}
	service := NewWithDeps("ffplay", nil, runner, recorder)

	err := service.Play(context.Background(), browse.SceneItem{
		ID:   42,
		Path: "/tmp/scene.mp4",
	}, nil)
	if err == nil {
		t.Fatal("expected ffplay error")
	}
	if recorder.called != 0 {
		t.Fatalf("recorder called=%d, want 0", recorder.called)
	}
}

func TestServiceRecordsResumeTimeFromPlayerProgress(t *testing.T) {
	pos := 125.5
	runner := &fakeRunner{pos: &pos}
	recorder := &fakeRecorder{}
	service := NewWithDeps("mpv", nil, runner, recorder)

	err := service.Play(context.Background(), browse.SceneItem{
		ID:   42,
		Path: "/tmp/scene.mp4",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if recorder.activityID != 42 {
		t.Fatalf("activityID = %d, want 42", recorder.activityID)
	}
	if recorder.resumeTime == nil || *recorder.resumeTime != pos {
		t.Fatalf("resumeTime = %v, want %v", recorder.resumeTime, pos)
	}
	if recorder.playDuration == nil {
		t.Fatal("playDuration should be recorded")
	}
}

func TestServiceStartsMPVAtResumeTime(t *testing.T) {
	runner := &fakeRunner{}
	service := NewWithDeps("mpv", []string{"--force-window=yes"}, runner, &fakeRecorder{})

	err := service.Play(context.Background(), browse.SceneItem{
		ID:         42,
		Path:       "/tmp/scene.mp4",
		ResumeTime: 125.5,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"--force-window=yes", "--start=125.5", "/tmp/scene.mp4"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("runner args = %#v, want %#v", runner.args, want)
	}
}

func TestParseMPVTimePos(t *testing.T) {
	got, ok := parseMPVTimePos([]byte(`{"event":"property-change","id":1,"name":"time-pos","data":125.5}`))
	if !ok {
		t.Fatal("expected time-pos event")
	}
	if got != 125.5 {
		t.Fatalf("time-pos = %v, want 125.5", got)
	}
}

func TestServiceRequiresVideoPath(t *testing.T) {
	service := NewWithDeps("ffplay", nil, &fakeRunner{}, &fakeRecorder{})

	err := service.Play(context.Background(), browse.SceneItem{ID: 42}, nil)
	if err == nil {
		t.Fatal("expected missing path error")
	}
}
