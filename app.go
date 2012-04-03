package visor

import (
	"fmt"
	"strings"
	"time"
)

const APPS_PATH = "apps"
const DEPLOY_LXC = "lxc"

type App struct {
	Snapshot
	Name       string
	RepoUrl    string
	Stack      Stack
	DeployType string
}

// NewApp returns a new App given a name, repository url and stack.
func NewApp(name string, repourl string, stack Stack, snapshot Snapshot) (app *App, err error) {
	app = &App{Name: name, RepoUrl: repourl, Stack: stack, Snapshot: snapshot}
	return
}

func (a *App) createSnapshot(rev int64) (app Snapshotable) {
	app = &App{Name: a.Name, RepoUrl: a.RepoUrl, Stack: a.Stack, Snapshot: Snapshot{rev, a.conn}}
	return
}

// FastForward advances the application in time. It returns
// a new instance of Application with the supplied revision.
func (a *App) FastForward(rev int64) (app *App) {
	return a.Snapshot.fastForward(a, rev).(*App)
}

// Register adds the App to the global process state.
func (a *App) Register() (app *App, err error) {
	exists, _, err := a.conn.Exists(a.Path(), &a.Rev)
	if err != nil {
		return
	}
	if exists {
		return nil, ErrKeyConflict
	}

	_, err = a.setPath("registered", time.Now().UTC().String())
	if err != nil {
		return
	}

	if a.DeployType == "" {
		a.DeployType = DEPLOY_LXC
	}

	attrs := &File{a.Snapshot, a.Path() + "/attrs", map[string]string{
		"repo-url":    a.RepoUrl,
		"stack":       string(a.Stack),
		"deploy-type": a.DeployType,
	}, new(JSONCodec)}

	f, err := attrs.Create()
	if err != nil {
		return
	}

	app = a.FastForward(f.Rev)

	return
}

// Unregister removes the App form the global process state.
func (a *App) Unregister() error {
	return a.conn.Del(a.Path(), a.Rev)
}

// EnvironmentVars returns all set variables for this app as a map.
func (a *App) EnvironmentVars() (vars map[string]string, err error) {
	varNames, err := a.conn.Getdir(a.Path()+"/env", a.Rev)
	if err != nil {
		return
	}

	vars = map[string]string{}
	var v string

	for i := range varNames {
		v, err = a.GetEnvironmentVar(varNames[i])
		if err != nil {
			return
		}

		vars[varNames[i]] = v
	}

	return
}

// GetEnvironmentVar returns the value stored for the given key.
func (a *App) GetEnvironmentVar(k string) (value string, err error) {
	val, _, err := a.conn.Get(a.Path()+"/env/"+k, &a.Rev)
	if err != nil {
		return
	}
	value = string(val)

	return
}

// SetEnvironmentVar stores the value for the given key.
func (a *App) SetEnvironmentVar(k string, v string) (app *App, err error) {
	rev, err := a.setPath("env/"+k, v)
	if err != nil {
		return
	}
	app = a.FastForward(rev)
	return
}

// DelEnvironmentVar removes the env variable for the given key.
func (a *App) DelEnvironmentVar(k string) (app *App, err error) {
	err = a.delPath("env/" + k)
	if err != nil {
		return
	}
	app = a.FastForward(a.Rev + 1)
	return
}

func (a *App) prefixPath(path string) string {
	return strings.Join([]string{a.Path(), path}, "/")
}

func (a *App) String() string {
	return fmt.Sprintf("%#v", a)
}

// Path returns the path for the App in the global process state.
func (a *App) Path() (p string) {
	return strings.Join([]string{APPS_PATH, a.Name}, "/")
}

func (a *App) setPath(k string, v string) (rev int64, err error) {
	return a.conn.Set(a.prefixPath(k), a.Rev, []byte(v))
}
func (a *App) delPath(k string) error {
	return a.conn.Del(a.prefixPath(k), a.Rev)
}

// GetApp fetches an app with the given name.
func GetApp(s Snapshot, name string) (app *App, err error) {
	app, err = NewApp(name, "", "", s)
	if err != nil {
		return nil, err
	}

	f, err := Get(s, app.Path()+"/attrs", new(JSONCodec))
	if err != nil {
		return nil, err
	}

	value := f.Value.(map[string]interface{})

	app.RepoUrl = value["repo-url"].(string)
	app.Stack = Stack(value["stack"].(string))
	app.DeployType = value["deploy-type"].(string)

	return
}

// Apps returns the list of all registered Apps.
func Apps(s Snapshot) (apps []*App, err error) {
	names, err := s.conn.Getdir(APPS_PATH, s.Rev)
	if err != nil {
		return
	}
	apps = make([]*App, len(names))

	for i := range names {
		a, e := GetApp(s, names[i])
		if err != nil {
			return nil, e
		}
		apps[i] = a
	}
	return
}