// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func ticketSetup() (s Snapshot, hostname string) {
	s, err := Dial(DEFAULT_ADDR, "/ticket-test")
	if err != nil {
		panic(err)
	}
	hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}

	s.conn.Del("tickets", s.Rev)
	s = s.FastForward(-1)

	return
}

func TestTicketCreateTicket(t *testing.T) {
	s, _ := ticketSetup()

	ticket, err := CreateTicket("lol", "cat", "app", OpStart, s)
	if err != nil {
		t.Error(err)
	}

	b, _, err := s.conn.Get(ticket.Path()+"/op", &ticket.Snapshot.Rev)
	if err != nil {
		t.Error(err)
	}

	body := "lol cat app start"
	if string(b) != body {
		t.Errorf("expected %s got %s", body, string(b))
	}
}

func TestTicketClaim(t *testing.T) {
	s, host := ticketSetup()
	id := s.Rev

	ticket, err := CreateTicket("claim", "abcd123", "test", OpStart, s)
	if err != nil {
		t.Error(err)
	}

	ticket, err = ticket.Claim(host)
	if err != nil {
		t.Error(err)
	}

	status, _, err := ticket.conn.Get("tickets/"+strconv.FormatInt(id, 10)+"/status", &ticket.Rev)
	if err != nil {
		t.Error(err)
	}
	if TicketStatus(status) != TicketStatusClaimed {
		t.Error("Ticket not claimed")
	}

	exists, _, err := ticket.conn.Exists("tickets/" + strconv.FormatInt(id, 10) + "/claims/" + host)
	if !exists {
		t.Error(err)
	}

	claims, err := ticket.Claims()
	if err != nil {
		t.Error(err)
	}
	if len(claims) != 1 {
		t.Error("claims/ folder expect to contain exactly one file")
	}

	_, err = ticket.Claim(host)
	if err != ErrTicketClaimed {
		t.Error("Ticket claimed twice")
	}
}

func TestTicketUnclaim(t *testing.T) {
	s, host := ticketSetup()
	id := s.Rev
	ticket := &Ticket{Id: id, AppName: "unclaim", RevisionName: "abcd123", ProcessName: "test", Op: OpStart, Snapshot: s}

	rev, err := s.conn.Set("tickets/"+strconv.FormatInt(id, 10)+"/claims/"+host, s.Rev, []byte(host))
	if err != nil {
		t.Error(err)
	}

	ticket.Snapshot = ticket.Snapshot.FastForward(rev)
	err = ticket.Unclaim(host)
	if err != nil {
		t.Error(err)
	}

	val, _, err := s.conn.Get("tickets/"+strconv.FormatInt(id, 10)+"/status", nil)
	if err != nil {
		t.Error(err)
	}
	if string(val) != "unclaimed" {
		t.Error("ticket still claimed")
	}
}

func TestTicketUnclaimWithWrongLock(t *testing.T) {
	s, host := ticketSetup()
	p := "tickets/" + strconv.FormatInt(s.Rev, 10) + "/claims/" + host
	ticket := &Ticket{Id: s.Rev, AppName: "unclaim", RevisionName: "abcd123", ProcessName: "test", Op: OpStart, Snapshot: s}

	rev, err := s.conn.Set(p, s.Rev, []byte(host))
	if err != nil {
		t.Error(err)
	}

	ticket.Snapshot = ticket.Snapshot.FastForward(rev)
	err = ticket.Unclaim("foo.bar.local")
	if err != ErrUnauthorized {
		t.Error("ticket unclaimed with wrong lock")
	}
}

func TestTicketDone(t *testing.T) {
	s, host := ticketSetup()
	p := "tickets/" + strconv.FormatInt(s.Rev, 10)
	ticket := &Ticket{Id: s.Rev, AppName: "done", RevisionName: "abcd123", ProcessName: "test", Op: OpStart, Snapshot: s}

	rev, err := s.conn.Set(p+"/claims/"+host, s.Rev, []byte(host))
	if err != nil {
		t.Error(err)
	}
	ticket.Snapshot = ticket.Snapshot.FastForward(rev)

	err = ticket.Done(host)
	if err != nil {
		t.Error(err)
	}

	exists, _, err := s.conn.Exists(p)
	if err != nil {
		t.Error(err)
	}
	if exists {
		t.Error("ticket not resolved")
	}
}

func TestTicketDoneWithWrongLock(t *testing.T) {
	s, host := ticketSetup()
	p := "tickets/" + strconv.FormatInt(s.Rev, 10)
	ticket := &Ticket{Id: s.Rev, AppName: "done", RevisionName: "abcd123", ProcessName: "test", Op: OpStart, Snapshot: s}

	_, err := s.conn.Set(p+"/claims/"+host, s.Rev, []byte(host))
	if err != nil {
		t.Error(err)
	}
	ticket.Snapshot = ticket.Snapshot.FastForward(-1)

	err = ticket.Done("foo.bar.local")
	if err != ErrUnauthorized {
		t.Error("ticket resolved with wrong lock")
	}
}

func TestTicketWatchCreate(t *testing.T) {
	s, _ := ticketSetup()
	l := make(chan *Ticket)

	go WatchTicket(s, l)

	_, err := CreateTicket("lol", "cat", "app", OpStart, s)
	if err != nil {
		t.Error(err)
	}

	expectTicket("lol", "cat", "app", OpStart, l, t)
}

func TestTicketWatchUnclaim(t *testing.T) {
	s, _ := ticketSetup()
	l := make(chan *Ticket)

	ticket, err := CreateTicket("lol", "cat", "app", OpStart, s)
	if err != nil {
		t.Error(err)
	}

	go WatchTicket(ticket.Snapshot, l)

	ticket, err = ticket.Claim("host")
	if err != nil {
		t.Error(err)
		return
	}

	err = ticket.Unclaim("host")
	if err != nil {
		t.Error(err)
		return
	}
	expectTicket("lol", "cat", "app", OpStart, l, t)
}

func expectTicket(appName, revName, pName string, op OperationType, l chan *Ticket, t *testing.T) {
	for {
		select {
		case ticket := <-l:
			if ticket.AppName == appName &&
				ticket.RevisionName == revName &&
				string(ticket.ProcessName) == pName &&
				ticket.Op == op {
				return
			} else {
				t.Errorf("received unexpected ticket: %s", ticket.String())
				return
			}
		case <-time.After(time.Second):
			t.Errorf("expected ticket, got timeout")
			return
		}
	}
}
