// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"code.google.com/p/goprotobuf/proto"
	"encoding/json"
	"fmt"
	"github.com/soundcloud/visor/generated"
	"time"
)

// A Revision represents an application revision,
// identifiable by its `ref`.
type Revision struct {
	Dir   dir
	App   *App
	Ref   string
	state *generated.Revision
}

const (
	pathRevisionRoot             = "revs"
	pathRevisionEntityDefinition = "entity-definition"

	// Pending Deprecation:
	pathRevisionRegistered = "registered"
	pathRevisionArchiveUrl = "archive-url"
)

const (
	archiveUrlUnavailable = "<unavailable>"
)

// NewRevision returns a new instance of Revision.
func NewRevision(app *App, ref string, snapshot Snapshot) *Revision {
	return &Revision{
		App: app,
		Ref: ref,
		Dir: dir{
			Snapshot: snapshot,
			Name:     app.Dir.prefix(pathRevisionRoot, ref),
		},
		state: &generated.Revision{
			State: generated.Revision_PROPOSED.Enum(),
		},
	}
}

func (r *Revision) createSnapshot(rev int64) snapshotable {
	tmp := *r
	tmp.Dir.Snapshot = Snapshot{
		Rev:  rev,
		conn: r.Dir.Snapshot.conn,
	}
	return &tmp
}

// FastForward advances the revision in time. It returns
// a new instance of Revision with the supplied revision.
func (r *Revision) FastForward(rev int64) *Revision {
	return r.Dir.Snapshot.fastForward(r, rev).(*Revision)
}

// Pending Deprecation:
func (r *Revision) upgrade() (revision *Revision, err error) {
	root := r.Dir.Name
	revision = r

	if revision.state.State == nil {
		revision.state.State = generated.Revision_ACCEPTED.Enum()
	}

	exists, _, err := revision.Dir.Snapshot.conn.Exists(root + "/" + pathRevisionArchiveUrl)
	if err != nil {
		return
	}

	if exists {
		v := "<undefined>"
		v, _, err = revision.Dir.get(pathRevisionArchiveUrl)
		if err != nil {
			return
		}

		revision.state.ArchiveUrl = proto.String(v)

		revision, err = revision.put()
		if err != nil {
			return
		}

		err = revision.Dir.del(pathRevisionArchiveUrl)
		if err != nil {
			return
		}
	}

	exists, _, err = revision.Dir.Snapshot.conn.Exists(root + "/" + pathRevisionRegistered)
	if err != nil {
		return
	}

	if exists {
		v := "<undefined>"
		v, _, err = revision.Dir.get(pathRevisionRegistered)
		if err != nil {
			return
		}

		t := time.Time{}

		t, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return
		}

		revision.state.RegistrationTimestamp = proto.Int64(t.Unix())

		revision, err = revision.put()
		if err != nil {
			return
		}

		err = revision.Dir.del(pathRevisionRegistered)
		if err != nil {
			return
		}
	}

	return
}

func (r *Revision) put() (revision *Revision, err error) {
	if marshaled, err := json.Marshal(r.state); err == nil {
		if rev, err := r.Dir.setBytes(pathRevisionEntityDefinition, marshaled); err == nil {
			revision = r.FastForward(rev)
		}
	}

	return
}

// Register registers a new Revision with the registry.
func (r *Revision) Propose() (revision *Revision, err error) {
	if *r.state.State != generated.Revision_PROPOSED {
		return nil, fmt.Errorf("Revision %s in state %s cannot be proposed", r, r.state.State)
	}

	r.state.RegistrationTimestamp = proto.Int64(time.Now().Unix())

	exists, _, err := r.Dir.Snapshot.conn.Exists(r.Dir.Name)
	if err != nil {
		return
	}
	if exists {
		return nil, ErrKeyConflict
	}

	revision, err = r.put()

	return
}

func (r *Revision) Accept(archiveUrl string) (revision *Revision, err error) {
	if *r.state.State != generated.Revision_PROPOSED {
		return nil, fmt.Errorf("Revision %s in state %s cannot be ratified", r, r.state.State)
	}

	r.state.State = generated.Revision_ACCEPTED.Enum()

	r.state.ArchiveUrl = proto.String(archiveUrl)

	revision, err = r.put()

	return
}

// Unregister unregisters a revision from the registry.
func (r *Revision) Purge() (err error) {
	// TODO(mtp): Invalidate purges against revisions with scale > 0.
	r.state.State = generated.Revision_REJECTED.Enum()

	return r.Dir.del("/")
}

func (r *Revision) String() string {
	return fmt.Sprintf("Revision<%s:%s>", r.App.Name, r.Ref)
}

func (r *Revision) Inspect() string {
	return fmt.Sprintf("%#v", r)
}

func GetRevision(s Snapshot, app *App, ref string) (r *Revision, err error) {
	path := app.Dir.prefix(pathRevisionRoot, ref)

	dir := dir{
		Snapshot: s,
		Name:     path,
	}

	if marshaled, _, err := dir.getBytes(pathRevisionEntityDefinition); err == nil {
		unmarshaled := &generated.Revision{}

		if err := json.Unmarshal(marshaled, unmarshaled); err == nil {
			r = &Revision{
				Dir:   dir,
				App:   app,
				Ref:   ref,
				state: unmarshaled,
			}
		}
	}

	// Pending Deprecation:
	r, err = r.upgrade()

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

func (r *Revision) State() string {
	s := int32(*r.state.State)
	n, _ := generated.Revision_State_name[s]
	return n
}

func (r *Revision) ArchiveUrl() string {
	if *r.state.State != generated.Revision_ACCEPTED {
		return archiveUrlUnavailable
	}

	return *r.state.ArchiveUrl
}

func (r *Revision) RegistrationTimestamp() time.Time {
	return time.Unix(*r.state.RegistrationTimestamp, 0)
}

func (r *Revision) IsScalable() (s bool) {
	return *r.state.State == generated.Revision_ACCEPTED
}

// AppRevisions returns an array of all registered revisions belonging
// to the given application.
func AppRevisions(s Snapshot, app *App) (revisions []*Revision, err error) {
	revs, err := s.getdir(app.Dir.prefix(pathRevisionRoot))
	if err != nil {
		return
	}

	results, err := getSnapshotables(revs, func(name string) (snapshotable, error) {
		return GetRevision(s, app, name)
	})
	if err != nil {
		return nil, err
	}

	for _, r := range results {
		revisions = append(revisions, r.(*Revision))
	}
	return
}
