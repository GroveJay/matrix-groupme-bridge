package faye

import "log"

type fayeDefaultLogger struct{}

func (l fayeDefaultLogger) Infof(f string, a ...interface{}) {
	log.Printf("[INFO]  : "+f, a...)
}
func (l fayeDefaultLogger) Errorf(f string, a ...interface{}) {
	log.Printf("[ERROR] : "+f, a...)
}
func (l fayeDefaultLogger) Debugf(f string, a ...interface{}) {
	log.Printf("[DEBUG] : "+f, a...)
}
func (l fayeDefaultLogger) Warnf(f string, a ...interface{}) {
	log.Printf("[WARN]  : "+f, a...)
}
