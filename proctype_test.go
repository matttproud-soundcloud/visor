// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"errors"
	"testing"
)

func proctypeSetup(appid string) (s Snapshot, app *App) {
	s, err := Dial(DefaultAddr, "/proctype-test")
	if err != nil {
		panic(err)
	}

	r, _ := s.conn.Rev()
	s.conn.Del("/", r)
	s = s.FastForward(-1)

	r, err = Init(s)
	if err != nil {
		panic(err)
	}
	s = s.FastForward(r)

	app = NewApp(appid, "git://proctype.git", "master", s)

	return
}

func TestProcTypeRegister(t *testing.T) {
	s, app := proctypeSetup("reg123")
	pty := NewProcType(app, "whoop", s)

	pty, err := pty.Register()
	if err != nil {
		t.Error(err)
	}

	check, _, err := s.conn.Exists(pty.Dir.Name)
	if err != nil {
		t.Error(err)
	}
	if !check {
		t.Errorf("proctype %s isn't registered", pty)
	}
}

func TestProcTypeRegisterWithInvalidName1(t *testing.T) {
	s, app := proctypeSetup("reg1232")
	pty := NewProcType(app, "who-op", s)

	pty, err := pty.Register()
	if err != ErrBadPtyName {
		t.Errorf("invalid proc type name (who-op) did not raise error")
	}
	if err != ErrBadPtyName && err != nil {
		t.Fatal("wrong error was raised for invalid proc type name")
	}
}

func TestProcTypeRegisterWithInvalidName2(t *testing.T) {
	s, app := proctypeSetup("reg1233")
	pty := NewProcType(app, "who_op", s)

	pty, err := pty.Register()
	if err != ErrBadPtyName {
		t.Errorf("invalid proc type name (who_op) did not raise error")
	}
	if err != ErrBadPtyName && err != nil {
		t.Fatal("wrong error was raised for invalid proc type name")
	}
}

func TestProcTypeUnregister(t *testing.T) {
	s, app := proctypeSetup("unreg123")
	pty := NewProcType(app, "whoop", s)

	pty, err := pty.Register()
	if err != nil {
		t.Error(err)
	}

	err = pty.Unregister()
	if err != nil {
		t.Error(err)
	}

	check, _, err := s.exists(pty.Dir.Name)
	if check {
		t.Errorf("proctype %s is still registered", pty)
	}
}

func TestProcTypeGetInstances(t *testing.T) {
	appid := "get-instances-app"
	s, app := proctypeSetup(appid)

	pty := NewProcType(app, "web", s)
	pty, err := pty.Register()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		ins, err := RegisterInstance(appid, "128af90", "web", s)
		if err != nil {
			t.Fatal(err)
		}
		ins, err = ins.Claim("10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		ins, err = ins.Start("10.0.0.1", 9999, appid+".org")
		if err != nil {
			t.Fatal(err)
		}
		pty = pty.FastForward(ins.Dir.Snapshot.Rev)
	}

	is, err := pty.GetInstances()
	if err != nil {
		t.Fatal(err)
	}
	if len(is) != 3 {
		t.Errorf("list is missing instances: %s", is)
	}
}

func TestProcTypeGetFailedInstances(t *testing.T) {
	appid := "get-failed-instances-app"
	s, app := proctypeSetup(appid)

	pty := NewProcType(app, "web", s)
	pty, err := pty.Register()
	if err != nil {
		t.Fatal(err)
	}

	instances := []*Instance{}

	for i := 0; i < 7; i++ {
		ins, err := RegisterInstance(appid, "128af9", "web", s)
		if err != nil {
			t.Fatal(err)
		}
		ins, err = ins.Claim("10.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		ins, err = ins.Start("10.0.0.1", 9999, appid+".org")
		if err != nil {
			t.Fatal(err)
		}
		instances = append(instances, ins)
		pty = pty.FastForward(ins.Dir.Snapshot.Rev)
	}
	for i := 0; i < 4; i++ {
		ins, err := instances[i].Failed("10.0.0.1", errors.New("no reason."))
		if err != nil {
			t.Fatal(err)
		}
		pty = pty.FastForward(ins.Dir.Snapshot.Rev)
	}

	is, err := pty.GetFailedInstances()
	if err != nil {
		t.Fatal(err)
	}
	if len(is) != 4 {
		t.Errorf("list is missing instances: %s", is)
	}
}
