// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"errors"
	"testing"
	"time"
)

func instanceSetup() (s Snapshot) {
	s, err := Dial(DefaultAddr, "/instance-test")
	if err != nil {
		panic(err)
	}

	r, _ := s.conn.Rev()
	s.conn.Del("/", r)
	s = s.FastForward(-1)

	return
}

func instanceSetupClaimed(name, host string) (i *Instance) {
	s := instanceSetup()

	i, err := RegisterInstance(name, "128af9", "web", s)
	if err != nil {
		panic(err)
	}

	i, err = i.Claim(host)
	if err != nil {
		panic(err)
	}
	return
}

func TestInstanceRegisterAndGet(t *testing.T) {
	s := instanceSetup()

	ins, err := RegisterInstance("cat", "128af9", "web", s)
	if err != nil {
		t.Fatal(err)
	}

	if ins.Status != InsStatusPending {
		t.Error("instance status wasn't set correctly")
	}
	if ins.Id <= 0 {
		t.Error("instance id wasn't set correctly")
	}

	ins1, err := GetInstance(ins.Dir.Snapshot, ins.Id)
	if err != nil {
		t.Fatal(err)
	}

	if ins1.Id != ins.Id {
		t.Error("ids don't match")
	}
	if ins1.Status != ins.Status {
		t.Error("statuses don't match")
	}
}

func TestInstanceClaiming(t *testing.T) {
	host := "10.0.0.1"
	s := instanceSetup()

	ins, err := RegisterInstance("bat", "128af9", "web", s)
	if err != nil {
		t.Fatal(err)
	}

	ins1, err := ins.Claim(host)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ins.Claim(host) // Already claimed
	if err.(*Error).Err != ErrRevMismatch {
		t.Error("expected re-claim to fail")
	}

	_, err = ins1.Claim(host) // Already claimed
	if err != ErrInsClaimed {
		t.Error("expected re-claim to fail")
	}

	claims, err := ins1.Claims()
	if err != nil {
		t.Fatal(err)
	}

	if len(claims) == 0 {
		t.Error("instance claim was unsuccessful")
	}
	if claims[0] != host {
		t.Error("instance claims doesn't include claimer")
	}

	ins2, err := ins1.Unclaim(host)
	if err != nil {
		t.Fatal(err)
	}

	claimer, err := ins2.getClaimer()
	if err != nil {
		t.Fatal(err)
	}
	if claimer != nil {
		t.Error("ticket wasn't unclaimed properly")
	}

	_, err = ins1.Unclaim("9.9.9.9") // Wrong host
	if err != ErrUnauthorized {
		t.Error("expected unclaim to fail")
	}

	_, err = ins2.Unclaim(host) // Already unclaimed
	if err != ErrUnauthorized {
		t.Error("expected unclaim to fail")
	}
}

func TestInstanceStarted(t *testing.T) {
	appid := "fat"
	ptyid := "web"
	revid := "128af9"
	ip := "10.0.0.1"
	port := 25790
	host := "fat.the-pink-rabbit.co"
	s := instanceSetup()

	ins, err := RegisterInstance(appid, revid, ptyid, s)
	if err != nil {
		t.Fatal(err)
	}
	ins1, err := ins.Claim(ip)
	if err != nil {
		t.Fatal(err)
	}

	ins2, err := ins1.Start(ip, port, host)
	if err != nil {
		t.Fatal(err)
	}

	if ins2.Status != InsStatusRunning {
		t.Errorf("unexpected status '%s'", ins2.Status)
	}

	if ins2.Port != port || ins2.Host != host || ins2.Ip != ip {
		t.Errorf("instance attributes not set correctly for %#v", ins2)
	}

	ins3, err := GetInstance(ins2.Dir.Snapshot, ins2.Id)
	if err != nil {
		t.Fatal(err)
	}
	if ins3.Port != port || ins3.Host != host || ins3.Ip != ip {
		t.Errorf("instance attributes not stored correctly for %#v", ins3)
	}

	ids, err := getInstanceIds(ins2.Dir.Snapshot, appid, revid, ptyid)
	if err != nil {
		t.Fatal(err)
	}

	if !func() bool {
		for _, id := range ids {
			if id == ins.Id {
				return true
			}
		}
		return false
	}() {
		t.Errorf("instance wasn't found under proc '%s'", ptyid)
	}
}

func TestInstanceStop(t *testing.T) {
	ip := "10.0.0.1"
	s := instanceSetup()

	ins, err := RegisterInstance("rat", "128af9", "web", s)
	if err != nil {
		t.Fatal(err)
	}
	ins1, err := ins.Claim(ip)
	if err != nil {
		t.Fatal(err)
	}

	_, err = StopInstance(ins.Id, ins1.Dir.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// Note: we aren't checking that the files are created in the coordinator,
	// that is better tested via events in event.go, as we don't want to couple
	// the tests with the schema.
}

func TestInstanceExited(t *testing.T) {
	ip := "10.0.0.1"
	port := 25790
	host := "fat.the-pink-rabbit.co"
	s := instanceSetup()

	ins, err := RegisterInstance("rat-cat", "128af9", "web", s)
	if err != nil {
		t.Fatal(err)
	}
	ins1, err := ins.Claim(ip)
	if err != nil {
		t.Fatal(err)
	}

	ins2, err := ins1.Start(ip, port, host)
	if err != nil {
		t.Fatal(err)
	}

	//_, err = ins2.Exited(ip)
	//if err != ErrUnauthorized {
	//	t.Error("expected command to fail")
	//}

	s1, err := StopInstance(ins2.Id, ins2.Dir.Snapshot)
	if err != nil {
		t.Fatal(err)
	}

	ins3, err := ins2.FastForward(s1.Rev).Exited(ip)
	if err != nil {
		t.Fatal(err)
	}
	testInstanceStatus(t, ins.Id, InsStatusExited, ins3.Dir.Snapshot)
}

func TestInstanceFailed(t *testing.T) {
	ip := "10.0.0.1"
	ins := instanceSetupClaimed("fat-cat", ip)

	ins, err := ins.Start(ip, 9999, "fat-cat.com")
	if err != nil {
		t.Fatal(err)
	}

	ins1, err := ins.Failed(ip, errors.New("because."))
	if err != nil {
		t.Fatal(err)
	}
	testInstanceStatus(t, ins.Id, InsStatusFailed, ins1.Dir.Snapshot)

	_, err = ins.Failed("9.9.9.9", errors.New("no reason."))
	if err != ErrUnauthorized {
		t.Error("expected command to fail")
	}

	// Note: we do not test whether or not failed instances can be retrieved
	// here. See the proctype tests & (*Proctype).GetFailedInstances()
}

func TestWatchInstanceStartAndStop(t *testing.T) {
	appid := "w-app"
	revid := "w-rev"
	ptyid := "w-pty"
	s := instanceSetup()
	l := make(chan *Instance)

	go WatchInstanceStart(s, l, make(chan error))

	ins, err := RegisterInstance(appid, revid, ptyid, s)
	if err != nil {
		t.Error(err)
	}

	select {
	case ins = <-l:
		// TODO Check other fields
		if ins.AppName == appid && ins.RevisionName == revid && ins.ProcessName == ptyid {
			break
		}
		t.Errorf("received unexpected instance: %s", ins.String())
	case <-time.After(time.Second):
		t.Errorf("expected instance, got timeout")
	}

	// Stop test

	ch := make(chan *Instance)

	go func() {
		ins, err := ins.WaitStop()
		if err != nil {
			t.Fatal(err)
		}
		ch <- ins
	}()

	ins1, err := StopInstance(ins.Id, ins.Dir.Snapshot)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case ins2 := <-ch:
		if ins2 == nil {
			t.Error("instance is nil")
		}
		if ins1.Rev != ins2.Dir.Snapshot.Rev {
			t.Errorf("instance revs don't match: %d != %d", ins1.Rev, ins2.Dir.Snapshot.Rev)
		}
	case <-time.After(time.Second):
		t.Errorf("expected instance, got timeout")
	}
}

func TestInstanceWait(t *testing.T) {
	s := instanceSetup()
	ins, err := RegisterInstance("bob", "985245a", "web", s)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if _, err := ins.Claim("127.0.0.1"); err != nil {
			panic(err)
		}
	}()
	ins1, err := ins.WaitClaimed()
	if err != nil {
		t.Error(err)
	}
	if ins1.Status != InsStatusClaimed {
		t.Errorf("expected instance status to be %s", InsStatusClaimed)
	}

	go func() {
		if _, err := ins1.Start("127.0.0.1", 9000, "localhost"); err != nil {
			panic(err)
		}
	}()
	ins2, err := ins1.WaitStarted()
	if err != nil {
		t.Error(err)
	}
	if ins2.Status != InsStatusRunning {
		t.Errorf("expected instance status to be %s", InsStatusRunning)
	}
	if ins2.Ip != "127.0.0.1" || ins2.Port != 9000 || ins2.Host != "localhost" {
		t.Errorf("expected ip/port/host to match for %#v", ins2)
	}
}

func TestInstanceWaitStop(t *testing.T) {
	s := instanceSetup()
	ins, err := RegisterInstance("bobby", "985245a", "web", s)
	if err != nil {
		t.Fatal(err)
	}
	ins1, err := ins.Claim("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ins1.Start("127.0.0.1", 9000, "localhost"); err != nil {
		t.Fatal(err)
	}

	go func() {
		if _, err := StopInstance(ins.Id, s); err != nil {
			panic(err)
		}
	}()
	ins1, err = ins1.WaitStop()
	if err != nil {
		t.Fatal(err)
	}
	if ins1.Dir.Snapshot.Rev <= ins.Dir.Snapshot.Rev {
		t.Error("expected new revision to be greater than previous")
	}
}

func testInstanceStatus(t *testing.T, id int64, status InsStatus, s Snapshot) {
	ins, err := GetInstance(s, id)
	if err != nil {
		t.Fatal(err)
	}
	if ins.Status != status {
		t.Errorf("expected instance status to be '%s' got '%s'", status, ins.Status)
	}
}
