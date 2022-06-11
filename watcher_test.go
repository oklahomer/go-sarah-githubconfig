package githubconfig

import (
	"context"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/v2"
	"github.com/shurcooL/githubv4"
	"strconv"
	"testing"
	"time"
)

type DummyQuerier struct {
	QueryFunc func(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

func (dq *DummyQuerier) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	return dq.QueryFunc(ctx, q, variables)
}

func TestNewConfig(t *testing.T) {
	owner := "owner"
	name := "name"
	dir := "some/dir"

	config := NewConfig(owner, name, dir)

	if config.Owner != owner {
		t.Errorf("Passed owner is not set. Expected %s but was %s.", owner, config.Owner)
	}

	if config.Name != name {
		t.Errorf("Passed name is not set. Expected %s but was %s.", name, config.Name)
	}

	if config.BaseDir != dir {
		t.Errorf("Passed directory is not set. Expected %s but was %s.", dir, config.BaseDir)
	}

	if config.Branch == "" {
		t.Errorf("Default branch is not set.")
	}

	if config.Interval == 0 {
		t.Errorf("Default interval is not set.")
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		opts  []Option
		error bool
	}{
		{
			opts: []Option{
				func(w *watcher) {
					w.client = &DummyQuerier{}
				},
			},
			error: false,
		},
		{
			opts: []Option{
				func(w *watcher) {},
			},
			error: true,
		},
		{
			opts:  []Option{},
			error: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := &Config{
				Interval: time.Second,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			w, err := New(ctx, config, tt.opts...)
			if tt.error {
				if err == nil {
					t.Fatal("Expected error is not returned.")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error is returned: %s", err.Error())
			}

			actual, ok := w.(*watcher)
			if !ok {
				t.Fatalf("Unexpected typed value is returned: %T", w)
			}

			if actual.config != config {
				t.Errorf("Unexpected config value is stored: %+v", actual.config)
			}
		})
	}
}

func TestWatcher_Read(t *testing.T) {
	tests := []struct {
		config  *Config
		timeout bool
	}{
		{
			config: &Config{
				TimeOut: 100 * time.Millisecond,
			},
			timeout: false,
		},
		{
			config: &Config{
				TimeOut: 100 * time.Millisecond,
			},
			timeout: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := make(chan *request)
			w := &watcher{
				client:         nil,
				config:         tt.config,
				request:        req,
				subscription:   nil,
				unsubscription: nil,
			}

			expected := errors.New("dummy")
			go func() {
				select {
				case r := <-req:
					if !tt.timeout {
						r.err <- expected
					}

				case <-time.NewTimer(1 * time.Second).C:
					// Just to be sure goroutine does not leak
					return

				}
			}()

			var botType sarah.BotType = "bot"
			id := "id"
			out := struct{}{}
			e := w.Read(context.Background(), botType, id, out)

			if tt.timeout {
				if !errors.Is(e, SubscriptionTimeout) {
					t.Error("Expected timeout did not occur.")
				}
			} else {
				if !errors.Is(e, expected) {
					t.Errorf("Expected error is not returned: %+v", e)
				}
			}
		})
	}
}

func TestWatcher_Watch(t *testing.T) {
	w := &watcher{
		subscription: make(chan *subscription, 1),
	}

	var botType sarah.BotType = "bot"
	id := "id"
	callback := func() {}
	err := w.Watch(context.Background(), botType, id, callback)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s", err)
	}

	select {
	case s := <-w.subscription:
		if s.botType != botType {
			t.Errorf("Given BotType is not passed: %s", s.botType)
		}

		if s.id != id {
			t.Errorf("Given id is not passed: %s.", s.id)
		}

	case <-time.NewTimer(1 * time.Second).C:
		t.Error("Subscription is not passed.")

	}
}

func TestWatcher_Unwatch(t *testing.T) {
	w := &watcher{
		unsubscription: make(chan sarah.BotType, 1),
	}

	var botType sarah.BotType = "bot"
	err := w.Unwatch(botType)
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s", err)
	}

	select {
	case u := <-w.unsubscription:
		if u != botType {
			t.Errorf("Expected BotType is not passed: %s.", u)
		}

	case <-time.NewTimer(1 * time.Second).C:
		t.Error("Target BotType is not passed.")

	}
}

func TestWatcher_get(t *testing.T) {
	owner := "oklahomer"
	name := "go-sarah"
	dir := "bot/config"
	branch := "master"
	var botType sarah.BotType = "botType"
	id := "hello"
	ext := ".yml"
	oid := "oid"
	text := "name: oklahomer\nrole: member\n"
	querier := &DummyQuerier{QueryFunc: func(_ context.Context, q interface{}, v map[string]interface{}) error {
		typed, ok := q.(*query)
		if !ok {
			t.Fatalf("Given query is not type of *query: %T", q)
		}

		o := v["owner"].(githubv4.String)
		if string(o) != owner {
			t.Errorf("Expected 'owner' value of %s but was %s", owner, o)
		}

		n := v["name"].(githubv4.String)
		if string(n) != name {
			t.Errorf("Expected 'name' value of %s but was %s", name, n)
		}

		e := v["expression"].(githubv4.String)
		expectedExp := fmt.Sprintf("%s:%s/%s", branch, dir, botType)
		if string(e) != expectedExp {
			t.Errorf("Expected 'expression' value of %s but was %s", e, expectedExp)
		}

		typed.Repository.Object.Tree.Entries = []struct {
			Name   githubv4.String
			Object struct {
				Blob struct {
					Oid  githubv4.String
					Text githubv4.String
				} `graphql:"... on Blob"`
			}
		}{
			{
				Name: githubv4.String(fmt.Sprintf("%s%s", id, ext)),
				Object: struct {
					Blob struct {
						Oid  githubv4.String
						Text githubv4.String
					} `graphql:"... on Blob"`
				}{
					Blob: struct {
						Oid  githubv4.String
						Text githubv4.String
					}{
						Oid:  githubv4.String(oid),
						Text: githubv4.String(text),
					}},
			},
		}

		return nil
	}}
	w := &watcher{
		client: querier,
		config: &Config{
			Owner:    owner,
			Name:     name,
			BaseDir:  dir,
			Branch:   branch,
			Interval: 0,
			TimeOut:  0,
		},
	}

	files, err := w.get(context.Background(), botType)
	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	cfg, ok := files[id]
	if !ok {
		t.Fatalf("Expected configuration for id of %s is absent", id)
	}

	if cfg.id != id {
		t.Errorf("ID does not match: %s.", id)
	}

	if cfg.extension != ext {
		t.Errorf("Extension does not match: %s.", cfg.extension)
	}

	if cfg.content != text {
		t.Errorf("Content does not match: %s.", cfg.content)
	}
}

func TestWithClient(t *testing.T) {
	client := &githubv4.Client{}
	opt := WithClient(client)
	w := &watcher{}

	opt(w)

	if w.client != client {
		t.Error("Expected client is not set.")
	}
}

func TestWithToken(t *testing.T) {
	opt := WithToken(context.Background(), "foo")
	w := &watcher{}

	opt(w)

	if w.client == nil {
		t.Error("Client must be set with the given token")
	}
}
