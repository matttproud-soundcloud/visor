// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"testing"
	"time"
)

func revSetup() (s Snapshot, app *App) {
	s, err := Dial(DefaultAddr, "/revision-test")
	if err != nil {
		panic(err)
	}

	r, _ := s.conn.Rev()
	s.conn.Del("/", r)
	s = s.FastForward(-1)

	rev, err := Init(s)
	if err != nil {
		panic(err)
	}
	s = s.FastForward(rev)

	app = NewApp("rev-test", "git://rev.git", "references", s)

	return
}

func TestRevisionRegister(t *testing.T) {
	s, app := revSetup()
	rev := NewRevision(app, "stable", app.Dir.Snapshot)

	check, _, err := s.conn.Exists(rev.Dir.Name)
	if err != nil {
		t.Error(err)
		return
	}
	if check {
		t.Error("Revision already registered")
		return
	}

	rev, err = rev.Propose()
	if err != nil {
		t.Error(err)
		return
	}

	check, _, err = s.conn.Exists(rev.Dir.Name)
	if err != nil {
		t.Error(err)
	}
	if !check {
		t.Error("Revision registration failed")
	}

	_, err = rev.Propose()
	if err == nil {
		t.Error("Revision allowed to be registered twice")
	}

	if rev.State() != "PROPOSED" {
		t.Errorf("Revision must be in state PROPOSED")
	}

	if rev.IsScalable() {
		t.Error("An unpromoted revision should not be scalable")
	}
}

func TestRevisionRatify(t *testing.T) {
	s, app := revSetup()
	rev := NewRevision(app, "stable-to-be-ratified", app.Dir.Snapshot)

	check, _, err := s.conn.Exists(rev.Dir.Name)
	if err != nil {
		t.Error(err)
		return
	}
	if check {
		t.Error("Revision already registered")
		return
	}

	if rev.IsScalable() {
		t.Error("An unpromoted revision should not be scalable")
	}

	rev, err = rev.Propose()
	if err != nil {
		t.Error(err)
		return
	}

	if rev.IsScalable() {
		t.Error("An unpromoted revision should not be scalable")
	}

	check, _, err = s.conn.Exists(rev.Dir.Name)
	if err != nil {
		t.Error(err)
	}

	if !check {
		t.Error("Revision registration failed")
	}

	if rev.State() != "PROPOSED" {
		t.Errorf("Revision must be in state PROPOSED")
	}

	rev, err = rev.Accept("path/to/artifact")
	if err != nil {
		t.Error(err)
	}

	if rev.State() != "ACCEPTED" {
		t.Errorf("Revision must be in state ACCEPTED")
	}

	if rev.ArchiveUrl() != "path/to/artifact" {
		t.Errorf("Archive URL is incorrect")
	}

	if !rev.IsScalable() {
		t.Error("An promoted revision should be scalable")
	}
}

func TestRevisionUnregister(t *testing.T) {
	s, app := revSetup()
	rev := NewRevision(app, "master", app.Dir.Snapshot)

	rev, err := rev.Propose()
	if err != nil {
		t.Error(err)
	}

	err = rev.Purge()
	if err != nil {
		t.Error(err)
	}

	check, _, err := s.conn.Exists(rev.Dir.Name)
	if err != nil {
		t.Error(err)
	}
	if check {
		t.Error("Revision still registered")
	}

	if rev.IsScalable() {
		t.Error("Deleted revision should not be scalable")
	}
}

func TestRevisionUpgrade(t *testing.T) {
	_, app := revSetup()
	rev := NewRevision(app, "antique-pending-upgrade", app.Dir.Snapshot)
	rev.state.State = nil

	r, err := rev.Dir.set("archive-url", "path/to/artifact")
	if err != nil {
		t.Error(err)
	}

	rev = rev.FastForward(r)

	instant := time.Now()
	r, err = rev.Dir.set("registered", instant.Format(time.RFC3339))
	if err != nil {
		t.Error(err)
	}

	rev = rev.FastForward(r)

	rev, err = rev.upgrade()
	if err != nil {
		t.Error(err)
	}

	if rev.State() != "ACCEPTED" {
		t.Error("Upgraded revision was not in correct state")
	}

	if rev.ArchiveUrl() != "path/to/artifact" {
		t.Error("Upgraded artifact path is incorrect")
	}

	if rev.RegistrationTimestamp().Unix() != instant.Unix() {
		t.Error("Upgraded registration timestamp is incorrect")
	}

	if !rev.IsScalable() {
		t.Error("Antiques are expected to be scalable")
	}
}
