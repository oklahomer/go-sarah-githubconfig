package githubconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var SubscriptionTimeout = errors.New("timeout")

type Config struct {
	Owner    string        `json:"owner" yaml:"owner"`
	Name     string        `json:"name" yaml:"name"`
	BaseDir  string        `json:"base_dir" yaml:"base_dir"`
	Branch   string        `json:"branch" yaml:"branch"`
	Interval time.Duration `json:"interval" yaml:"interval"`
	TimeOut  time.Duration `json:"timeout" yaml:"timeout"`
}

func NewConfig(owner string, name string, baseDir string) *Config {
	return &Config{
		Owner:    owner,
		Name:     name,
		BaseDir:  baseDir,
		Branch:   "master",
		Interval: 1 * time.Minute,
		TimeOut:  5 * time.Second,
	}
}

type watcher struct {
	client         querier
	config         *Config
	request        chan *request
	subscription   chan *subscription
	unsubscription chan sarah.BotType
}

var _ sarah.ConfigWatcher = (*watcher)(nil)

func (w *watcher) Read(_ context.Context, botType sarah.BotType, id string, out interface{}) error {
	err := make(chan error)
	req := &request{
		botType: botType,
		id:      id,
		err:     err,
		out:     out,
	}
	w.request <- req

	select {
	case <-time.NewTimer(w.config.TimeOut).C:
		return SubscriptionTimeout

	case e := <-err:
		return e

	}
}

func (w *watcher) Watch(_ context.Context, botType sarah.BotType, id string, callback func()) error {
	s := &subscription{
		botType:  botType,
		id:       id,
		callback: callback,
	}
	w.subscription <- s
	return nil
}

func (w *watcher) Unwatch(botType sarah.BotType) error {
	w.unsubscription <- botType
	return nil
}

func (w *watcher) operate(ctx context.Context) {
	cache := map[sarah.BotType]map[string]*file{}
	subscription := map[sarah.BotType]map[string]func(){}

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case s := <-w.subscription:
			_, ok := subscription[s.botType]
			if !ok {
				subscription[s.botType] = map[string]func(){}
			}
			subscription[s.botType][s.id] = s.callback

		case botType := <-w.unsubscription:
			delete(cache, botType)
			delete(subscription, botType)

		case req := <-w.request:
			files, ok := cache[req.botType]
			if !ok {
				cache[req.botType] = map[string]*file{}

				files, err := w.get(ctx, req.botType)
				if err != nil {
					req.err <- err
					continue
				}
				cache[req.botType] = files
			}

			f := files[req.id]
			if f == nil {
				req.err <- &sarah.ConfigNotFoundError{
					BotType: req.botType,
					ID:      req.id,
				}
				continue
			}

			req.err <- read(f, req.out)

		case <-ticker.C:
			for botType, sub := range subscription {
				files, err := w.get(ctx, botType)
				if err != nil {
					// TODO logging
					continue
				}

				if _, ok := cache[botType]; !ok {
					cache[botType] = files
				}

				for id, callback := range sub {
					if f, ok := files[id]; ok {
						old, ok := cache[botType][f.id]
						if !ok || old.objectID != f.objectID {
							// Dispatch a goroutine to let the subscriber read the configuration.
							// In this way, a developer may call watcher.Read() in the callback.
							// A case with "<-w.request" blocks in watcher.Read() call, otherwise.
							go callback()
						}
					}
				}
				cache[botType] = files
			}

		}
	}
}

func read(f *file, out interface{}) error {
	switch f.extension {
	case ".yml", ".yaml":
		return yaml.Unmarshal([]byte(f.content), out)

	case ".json":
		return json.Unmarshal([]byte(f.content), out)

	default:
		return fmt.Errorf("unsupported file extension for %s: %s", f.id, f.extension)

	}
}

func (w *watcher) get(ctx context.Context, botType sarah.BotType) (map[string]*file, error) {
	q := &query{}
	dir := path.Join(w.config.BaseDir, botType.String())
	expression := fmt.Sprintf("%s:%s", w.config.Branch, strings.TrimPrefix(dir, "/"))
	variables := map[string]interface{}{
		"owner":      githubv4.String(w.config.Owner),
		"name":       githubv4.String(w.config.Name),
		"expression": githubv4.String(expression),
	}
	err := w.client.Query(ctx, q, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to query Github API: %w", err)
	}

	files := map[string]*file{}
	for _, entry := range q.Repository.Object.Tree.Entries {
		name := string(entry.Name)
		extension := filepath.Ext(name)
		id := strings.TrimSuffix(name, extension)
		cfg := &file{
			id:        id,
			fileName:  name,
			extension: extension,
			objectID:  string(entry.Object.Blob.Oid),
			content:   string(entry.Object.Blob.Text),
		}
		files[id] = cfg
	}
	return files, nil
}

func New(ctx context.Context, cfg *Config, opts ...Option) (sarah.ConfigWatcher, error) {
	w := &watcher{
		config:         cfg,
		request:        make(chan *request),
		subscription:   make(chan *subscription),
		unsubscription: make(chan sarah.BotType),
	}
	for _, opt := range opts {
		opt(w)
	}
	if w.client == nil {
		return nil, errors.New("githubv4.Client must be derived from WithClient or WithToken option")
	}

	go w.operate(ctx)

	return w, nil
}

type Option func(*watcher)

func WithClient(client *githubv4.Client) Option {
	return func(w *watcher) {
		w.client = client
	}
}

func WithToken(ctx context.Context, token string) Option {
	return func(w *watcher) {
		src := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient := oauth2.NewClient(ctx, src)
		w.client = githubv4.NewClient(httpClient)
	}
}

type subscription struct {
	botType  sarah.BotType
	id       string
	callback func()
}

type request struct {
	botType sarah.BotType
	id      string
	out     interface{}
	err     chan<- error
}

type querier interface {
	Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

// query represents a Graphql query to fetch configuration files.
// Formatted query is as below:
//
// 	query ($owner: String!, $name: String!, $expression:String!) {
//    repository(owner: $owner, name: $name) {
//      object(expression: $expression) {
//        ... on Tree {
//          entries {
//            name
//            object {
//              ... on Blob {
//                oid
//                text
//              }
//            }
//          }
//        }
//      }
//    }
// 	}
type query struct {
	Repository repository `graphql:"repository(owner: $owner, name: $name)"`
}

type repository struct {
	Object repositoryObject `graphql:"object(expression: $expression)"`
}

type repositoryObject struct {
	Tree tree `graphql:"... on Tree"`
}

type tree struct {
	Entries []entry
}

type entryObject struct {
	Blob blob `graphql:"... on Blob"`
}

type blob struct {
	Oid  githubv4.String
	Text githubv4.String
}

type entry struct {
	Name   githubv4.String
	Object entryObject
}

type file struct {
	id        string
	fileName  string
	extension string
	objectID  string
	content   string
}
