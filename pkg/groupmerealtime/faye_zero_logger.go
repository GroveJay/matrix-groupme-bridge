package groupmerealtime

import "github.com/rs/zerolog"

type FayeZeroLogger struct {
	zerolog.Logger
}

func (f FayeZeroLogger) Debugf(i string, a ...interface{}) {
	f.Logger.Debug().Msgf(i, a...)
}
func (f FayeZeroLogger) Errorf(i string, a ...interface{}) {
	f.Logger.Error().Msgf(i, a...)
}
func (f FayeZeroLogger) Warnf(i string, a ...interface{}) {
	f.Logger.Warn().Msgf(i, a...)
}
func (f FayeZeroLogger) Infof(i string, a ...interface{}) {
	f.Logger.Info().Msgf(i, a...)
}
