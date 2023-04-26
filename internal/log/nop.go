package log

// No-Op logger
type nopLogger struct{}

func (l *nopLogger) Errorf(string, ...interface{}) {}
func (l *nopLogger) Warnf(string, ...interface{})  {}
func (l *nopLogger) Infof(string, ...interface{})  {}
func (l *nopLogger) Info(...interface{})           {}
func (l *nopLogger) Debugf(string, ...interface{}) {}
func (l *nopLogger) Debug(...interface{})          {}
