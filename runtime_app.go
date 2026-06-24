package main

type runtimeApp struct {
	ID         string
	RootPath   string
	DataPath   string
	AppPath    string
	Process    string
	WorkingDir string
	Args       []string
	CommonArgs []string
	DisableLog bool
	close      func()
}

func (a *runtimeApp) Close() {
	if a == nil || a.close == nil {
		return
	}
	a.close()
}
