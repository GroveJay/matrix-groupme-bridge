package main

import (
	"context"
	"os"
	"time"

	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmeclient"
	"github.com/GroveJay/matrix-groupme-bridge/pkg/groupmerealtime"
	"github.com/rs/zerolog"
	"go.mau.fi/zeroconfig"
	"gopkg.in/yaml.v3"
)

func prepareLog(yamlConfig []byte) (*zerolog.Logger, error) {
	var cfg zeroconfig.Config
	err := yaml.Unmarshal(yamlConfig, &cfg)
	if err != nil {
		return nil, err
	}
	return cfg.Compile()
}

// This could be loaded from a file rather than hardcoded
const logConfig = `
min_level: trace
writers:
- type: stdout
  format: pretty-colored
`

func getMyUserID(client *groupmeclient.Client, ctx context.Context) groupmeclient.ID {
	user, err := client.MyUser(ctx)
	if err != nil {
		panic(err)
	}
	UserId := user.ID
	return UserId
}

func connectToPushAndSleepIndefinitely(ctx context.Context, logger *zerolog.Logger, UserId groupmeclient.ID, AuthToken string) {
	pushSubscription := groupmerealtime.NewPushSubscription(ctx)
	fullHandler := gha{
		logger: *logger,
	}
	pushSubscription.AddFullHandler(&fullHandler)
	fayeLogger := &groupmerealtime.FayeZeroLogger{Logger: *logger}
	err := pushSubscription.Setup(context.Background(), *groupmerealtime.NewFayeClient(*fayeLogger, AuthToken))
	if err != nil {
		logger.Error().Msgf("Got error from setup: %s", err)
		return
	}
	err = pushSubscription.SubscribeToUser(ctx, UserId)
	if err != nil {
		logger.Error().Msgf("Got error from subscribing to user: %s", err)
		return
	}
	for {
		time.Sleep(5 * time.Second)
	}
}

// First argument when running the test should be the auth token from GroupMe
func main() {
	ctx := context.Background()
	logger, err := prepareLog([]byte(logConfig))
	if err != nil {
		panic(err)
	}
	AuthToken := os.Args[1]
	client := groupmeclient.NewClient(AuthToken)
	UserId := getMyUserID(client, ctx)
	logger.Debug().Msgf("Got userID: %s", UserId)
	connectToPushAndSleepIndefinitely(ctx, logger, UserId, AuthToken)
}
