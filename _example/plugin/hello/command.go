// Package hello show how a configuration value for a sarah.Command can be live-updated.
//
// A Command with an identifier of "hello" is built with this example.
// Since its BotType is "slack," github.com/oklahomer/githubconfig's ConfigWatcher implementation expects a configuration file to be located under Config.BaseDir + "/slack/" directory.
// And since the identifier is "hello," the file name is expected to be one of hello.yml, hello.yaml, or hello.json.
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"strings"
)

type Config struct {
	Message string `yaml:"message" json:"message"`
}

func init() {
	config := &Config{
		Message: "Hello!",
	}
	props := sarah.NewCommandPropsBuilder().
		Identifier("hello").
		MatchFunc(func(input sarah.Input) bool {
			return strings.HasPrefix(input.Message(), ".hello")
		}).
		BotType(slack.SLACK).
		Instruction("Input .hello").
		ConfigurableFunc(config, func(_ context.Context, input sarah.Input, cfg sarah.CommandConfig) (*sarah.CommandResponse, error) {
			typed := cfg.(*Config)
			return slack.NewResponse(input, typed.Message)
		}).
		MustBuild()
	sarah.RegisterCommandProps(props)
}
