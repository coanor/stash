package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stashapp/stash/internal/cli/browse"
	"github.com/stashapp/stash/pkg/models"
)

type Runner interface {
	Run(context.Context, string, []string, func(float64)) error
}

type ViewRecorder interface {
	AddView(context.Context, int, time.Time) error
	SaveActivity(context.Context, int, *float64, *float64) error
}

type Service struct {
	path     string
	args     []string
	runner   Runner
	recorder ViewRecorder
}

func New(repo models.Repository, path string, args []string) *Service {
	return NewWithDeps(path, args, commandRunner{}, repoRecorder{repo: repo})
}

func NewWithDeps(path string, args []string, runner Runner, recorder ViewRecorder) *Service {
	return &Service{
		path:     strings.TrimSpace(path),
		args:     append([]string(nil), args...),
		runner:   runner,
		recorder: recorder,
	}
}

func (s *Service) Play(ctx context.Context, item browse.SceneItem, progress func(time.Duration)) error {
	if item.Path == "" {
		return errors.New("selected scene has no playable file path")
	}
	if s.path == "" {
		return errors.New("player is unavailable: configure player_path")
	}
	if s.runner == nil {
		return errors.New("player runner is not configured")
	}

	args := append([]string(nil), s.args...)
	args = appendResumeArg(args, s.path, item.ResumeTime)
	args = append(args, item.Path)
	started := time.Now()
	var mu sync.Mutex
	var lastPosition *float64
	if err := s.runner.Run(ctx, s.path, args, func(seconds float64) {
		mu.Lock()
		position := seconds
		lastPosition = &position
		mu.Unlock()
		if progress != nil {
			progress(secondsToDuration(seconds))
		}
	}); err != nil {
		return fmt.Errorf("play scene: %w", err)
	}
	if s.recorder != nil {
		if err := s.recorder.AddView(ctx, item.ID, time.Now()); err != nil {
			return fmt.Errorf("record scene view: %w", err)
		}
		mu.Lock()
		resumeTime := lastPosition
		mu.Unlock()
		if resumeTime != nil {
			playDuration := time.Since(started).Seconds()
			if err := s.recorder.SaveActivity(ctx, item.ID, resumeTime, &playDuration); err != nil {
				return fmt.Errorf("record scene activity: %w", err)
			}
		}
	}

	return nil
}

func appendResumeArg(args []string, playerPath string, resumeTime float64) []string {
	if resumeTime <= 0 {
		return args
	}
	value := strconv.FormatFloat(resumeTime, 'f', -1, 64)
	if isMPV(playerPath) {
		return append(args, "--start="+value)
	}
	return append(args, "-ss", value)
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, path string, args []string, progress func(float64)) error {
	if isMPV(path) {
		return runMPV(ctx, path, args, progress)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	return cmd.Run()
}

func isMPV(path string) bool {
	return strings.EqualFold(filepath.Base(path), "mpv")
}

func runMPV(ctx context.Context, path string, args []string, progress func(float64)) error {
	if progress == nil {
		cmd := exec.CommandContext(ctx, path, args...)
		return cmd.Run()
	}

	dir, err := os.MkdirTemp("", "stash-cli-mpv-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	socket := filepath.Join(dir, "mpv.sock")
	args = append(append([]string(nil), args...), "--input-ipc-server="+socket)
	cmd := exec.CommandContext(ctx, path, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go monitorMPV(ctx, socket, progress)
	return cmd.Wait()
}

func monitorMPV(ctx context.Context, socket string, progress func(float64)) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("unix", socket, 100*time.Millisecond)
		if err == nil {
			defer conn.Close()
			_, _ = fmt.Fprintln(conn, `{"command":["observe_property",1,"time-pos"]}`)
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				if seconds, ok := parseMPVTimePos(scanner.Bytes()); ok {
					progress(seconds)
				}
			}
			return
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func parseMPVTimePos(line []byte) (float64, bool) {
	var msg struct {
		Event string   `json:"event"`
		Name  string   `json:"name"`
		Data  *float64 `json:"data"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return 0, false
	}
	if msg.Event != "property-change" || msg.Name != "time-pos" || msg.Data == nil {
		return 0, false
	}
	return *msg.Data, true
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}

type repoRecorder struct {
	repo models.Repository
}

func (r repoRecorder) AddView(ctx context.Context, sceneID int, at time.Time) error {
	return r.repo.WithTxn(ctx, func(ctx context.Context) error {
		_, err := r.repo.Scene.AddViews(ctx, sceneID, []time.Time{at})
		return err
	})
}

func (r repoRecorder) SaveActivity(ctx context.Context, sceneID int, resumeTime *float64, playDuration *float64) error {
	return r.repo.WithTxn(ctx, func(ctx context.Context) error {
		_, err := r.repo.Scene.SaveActivity(ctx, sceneID, resumeTime, playDuration)
		return err
	})
}
