// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"fmt"
	"path"
	"time"
)

// A Revision represents an application revision,
// identifiable by its `ref`.
type Revision struct {
	Snapshot
	App        *App
	Ref        string
	ArchiveUrl string
}

const REVS_PATH = "revs"

// NewRevision returns a new instance of Revision.
func NewRevision(app *App, ref string, snapshot Snapshot) (rev *Revision) {
	rev = &Revision{App: app, Ref: ref, Snapshot: snapshot}

	return
}

func (r *Revision) createSnapshot(rev int64) Snapshotable {
	tmp := *r
	tmp.Snapshot = Snapshot{rev, r.conn}
	return &tmp
}

// FastForward advances the revision in time. It returns
// a new instance of Revision with the supplied revision.
func (r *Revision) FastForward(rev int64) *Revision {
	return r.Snapshot.fastForward(r, rev).(*Revision)
}

// Register registers a new Revision with the registry.
func (r *Revision) Register() (revision *Revision, err error) {
	exists, _, err := r.conn.Exists(r.Path())
	if err != nil {
		return
	}
	if exists {
		return nil, ErrKeyConflict
	}

	rev, err := r.conn.Set(r.Path()+"/archive-url", r.Rev, []byte(r.ArchiveUrl))
	if err != nil {
		return
	}
	rev, err = r.conn.Set(r.Path()+"/registered", r.Rev, []byte(time.Now().UTC().String()))
	if err != nil {
		return
	}

	revision = r.FastForward(rev)

	return
}

// Unregister unregisters a revision from the registry.
func (r *Revision) Unregister() (err error) {
	return r.conn.Del(r.Path(), r.Rev)
}

func (r *Revision) SetArchiveUrl(url string) (revision *Revision, err error) {
	rev, err := r.conn.Set(r.Path()+"/archive-url", r.Rev, []byte(url))
	if err != nil {
		return
	}
	revision = r.FastForward(rev)
	return
}

// Path returns this.Revision's directory path in the registry.
func (r *Revision) Path() string {
	return path.Join(r.App.Path(), REVS_PATH, r.Ref)
}

func (r *Revision) String() string {
	return fmt.Sprintf("Revision<%s:%s>", r.App.Name, r.Ref)
}

func (r *Revision) Inspect() string {
	return fmt.Sprintf("%#v", r)
}

func GetRevision(s Snapshot, app *App, ref string) (r *Revision, err error) {
	path := app.Path() + "/revs/" + ref
	codec := new(StringCodec)

	f, err := Get(s, path+"/archive-url", codec)
	if err != nil {
		return
	}

	r = &Revision{
		Snapshot:   s,
		App:        app,
		Ref:        ref,
		ArchiveUrl: f.Value.(string),
	}
	return
}

// Revisions returns an array of all registered revisions.
func Revisions(s Snapshot) (revisions []*Revision, err error) {
	apps, err := Apps(s)
	if err != nil {
		return
	}

	revisions = []*Revision{}

	for i := range apps {
		revs, e := AppRevisions(s, apps[i])
		if e != nil {
			return nil, e
		}
		revisions = append(revisions, revs...)
	}

	return
}

// AppRevisions returns an array of all registered revisions belonging
// to the given application.
func AppRevisions(s Snapshot, app *App) (revisions []*Revision, err error) {
	refs, err := s.conn.Getdir(app.Path()+"/revs", s.Rev)
	if err != nil {
		return
	}
	revisions = make([]*Revision, len(refs))

	for i := range refs {
		r, e := GetRevision(s, app, refs[i])
		if e != nil {
			return nil, e
		}

		revisions[i] = r
	}

	return
}
