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

const servicesPath = "services"

type Service struct {
	dir
	Name string
}

// NewService returns a new Service given a name.
func NewService(name string, snapshot Snapshot) (srv *Service) {
	srv = &Service{Name: name}
	srv.dir = dir{snapshot, path.Join(servicesPath, srv.Name)}

	return
}

func (s *Service) createSnapshot(rev int64) snapshotable {
	tmp := *s
	tmp.Snapshot = Snapshot{rev, s.conn}
	return &tmp
}

// FastForward advances the service in time. It returns
// a new instance of Service with the supplied revision.
func (s *Service) FastForward(rev int64) *Service {
	return s.Snapshot.fastForward(s, rev).(*Service)
}

// Register adds the Service to the global process state.
func (s *Service) Register() (srv *Service, err error) {
	exists, _, err := s.conn.Exists(s.dir.Name)
	if err != nil {
		return
	}
	if exists {
		return nil, ErrKeyConflict
	}

	rev, err := s.set("registered", time.Now().UTC().String())
	if err != nil {
		return
	}

	srv = s.FastForward(rev)

	return
}

// Unregister removes the Service form the global process state.
func (s *Service) Unregister() error {
	return s.del("/")
}

func (s *Service) String() string {
	return fmt.Sprintf("Service<%s>", s.Name)
}

func (s *Service) Inspect() string {
	return fmt.Sprintf("%#v", s)
}

func (s *Service) GetEndpoints() (endpoints []*Endpoint, err error) {
	p := s.dir.prefix(endpointsPath)

	exists, _, err := s.conn.Exists(p)
	if err != nil || !exists {
		return
	}

	ids, err := s.getdir(p)
	if err != nil {
		return
	}

	for _, id := range ids {
		var e *Endpoint

		e, err = GetEndpoint(s.Snapshot, s, id)
		if err != nil {
			return
		}

		endpoints = append(endpoints, e)
	}

	return
}

// GetService fetches a service with the given name.
func GetService(s Snapshot, name string) (srv *Service, err error) {
	service := NewService(name, s)

	exists, _, err := s.conn.Exists(service.dir.Name)
	if err != nil && !exists {
		return
	}

	srv = service

	return
}

// Services returns the list of all registered Services.
func Services(s Snapshot) (srvs []*Service, err error) {
	exists, _, err := s.conn.Exists(servicesPath)
	if err != nil || !exists {
		return
	}

	names, err := s.getdir(servicesPath)
	if err != nil {
		return
	}

	for _, name := range names {
		var srv *Service

		srv, err = GetService(s, name)
		if err != nil {
			return
		}

		srvs = append(srvs, srv)
	}

	return
}