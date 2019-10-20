# Introduction
This is a [`sarah.ConfigWatcher`](https://github.com/oklahomer/go-sarah/wiki/Live-Configuration-Update) implementation that subscribes to changes on GitHub repository.

`go-sarah`, by default, provides a `sarah.ConfigWatcher` implementation called [`fileWatcher`](https://github.com/oklahomer/go-sarah/blob/master/watchers/filewatcher.go) to reflect up-to-date configuration values for Command and ScheduledTask.
However, especially when bot instance is hosted on PasS, uploading configuration files to a specific location with or without re-build is restricted and hence fileWatcher is not the best choice.
In that case, set up a service such as [Consul](https://www.consul.io/) or [Central Dogma](https://line.github.io/centraldogma/) to host the configuration values and broadcast updates to its subscribers is one way to go but is a big leap for a simple personal bot instance.

This project, `github.com/oklahomer/go-sarah-githubconfig`, is another solution for such a scenario.
Its `sarah.ConfigWatcher` implementation subscribes to configuration files hosted on `GitHub` and applies the values to corresponding Command and ScheduledTasks.

Its construction is as below:
## Subscribing to GitHub repository
```go
    env := os.GetEnv("SARAH_ENV")
    repository := "go-sarah-blahblah-dev"
    if env == "production" {
        repository = "go-sarah-blahblah-prod"
    }
    cfg := githubconfig.NewConfig("oklahomer", repository, "bot/config")
    token := os.Getenv("GITHUB_TOKEN")
    watcher, err := githubconfig.New(ctx, cfg, githubconfig.WithToken(ctx, token))
```
With above settings, the `ConfigWatcher` will subscribe to `github.com/oklahomer/go-sarah-blahblah-(dev|prod)` repository's `bot/config/{BOT_TYPE}` directory.

## Subscribing to GitHub Enterprise repository
```go
    ctx := context.Background()
    token := os.Getenv("GITHUB_ENTERPRISE_TOKEN")
    src := oauth2.StaticTokenSource(
        &oauth2.Token{AccessToken: token}, 
    )
    httpClient := oauth2.NewClient(ctx, src)
    client := githubql.NewEnterpriseClient("https://example.com/git/api/graphql", httpClient)
    watcher, err := githubconfig.New(ctx, cfg, githubconfig.WithClient(client))
```

# How it works
Below depicts how the configuration value is reflected in a real-time manner without reboot.
![](/doc/img/sample.png)

When the `ConfigWatcher` realizes there is an update on the configuration file, this will read the file content and rebuild the corresponding Command or ScheduledTask with the new configuration value.
This will leave log messages as below:
```
sarah 2019/10/20 11:46:20 /Users/Oklahomer/go/pkg/mod/github.com/oklahomer/go-sarah/v2@v2.0.2/runner.go:430: [INFO] Updating command: hello
sarah 2019/10/20 11:46:20 /Users/Oklahomer/go/pkg/mod/github.com/oklahomer/go-sarah/v2@v2.0.2/command.go:168: [INFO] replacing old command in favor of newly appending one: hello.
```

# Example Codes
See [./example](https://github.com/oklahomer/go-sarah-githubconfig/blob/master/example/app/main.go) for example.
