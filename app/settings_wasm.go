//go:build wasm

package app

func (s *settings) load() {
	s.setupTheme()
	s.schema.Scale = 1
}

func (s *settings) loadFromFile(path string) error {
	return nil
}

func watchFile(path string, callback func()) any {
	return nil
}

func (s *settings) watchSettings() {
	watchTheme(s)
}

func (s *settings) stopWatching() {
	stopWatchingTheme()
}
