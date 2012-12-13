// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var reProcName = regexp.MustCompile("^[[:alnum:]]+$")

// ProcType represents a process type with a certain scale.
type ProcType struct {
	Dir  dir
	Name string
	App  *App
	Port int
}

const procsPath = "procs"

func NewProcType(app *App, name string, s Snapshot) *ProcType {
	return &ProcType{
		Name: name,
		App:  app,
		Dir: dir{
			s, app.Dir.prefix(procsPath, string(name)),
		},
	}
}

func (p *ProcType) createSnapshot(rev int64) snapshotable {
	tmp := *p
	tmp.Dir.Snapshot = Snapshot{rev, p.Dir.Snapshot.conn}
	return &tmp
}

// FastForward advances the instance in time. It returns
// a new instance of Instance with the supplied revision.
func (p *ProcType) FastForward(rev int64) *ProcType {
	return p.Dir.Snapshot.fastForward(p, rev).(*ProcType)
}

// Register registers a proctype with the registry.
func (p *ProcType) Register() (ptype *ProcType, err error) {
	exists, _, err := p.Dir.Snapshot.conn.Exists(p.Dir.Name)
	if err != nil {
		return
	}
	if exists {
		return nil, ErrKeyConflict
	}

	if !reProcName.MatchString(p.Name) {
		return nil, ErrBadPtyName
	}

	p.Port, err = ClaimNextPort(p.Dir.Snapshot)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't claim port: %s", err.Error()))
	}

	port := &file{p.Dir.Snapshot, -1, p.Dir.prefix("port"), p.Port, new(intCodec)}

	port, err = port.Create()
	if err != nil {
		return p, err
	}

	rev, err := p.Dir.set("registered", timestamp())

	if err != nil {
		return p, err
	}
	ptype = p.FastForward(rev)

	return
}

// Unregister unregisters a proctype from the registry.
func (p *ProcType) Unregister() (err error) {
	return p.Dir.del("/")
}

func (p *ProcType) instancesPath() string {
	return p.Dir.prefix(pathInstanceRoot)
}

func (p *ProcType) failedInstancesPath() string {
	return p.Dir.prefix(failedPath)
}

func (p *ProcType) NumInstances() (int, error) {
	revs, err := p.Dir.Snapshot.getdir(p.Dir.prefix("instances"))
	if err != nil {
		return -1, err
	}
	total := 0

	for _, rev := range revs {
		// TODO: remove this once all old instances are cleaned up
		if len(rev) != 7 {
			continue
		}
		size, _, err := p.Dir.Snapshot.conn.Stat(p.Dir.prefix("instances", rev), &p.Dir.Snapshot.Rev)
		if err != nil {
			return -1, err
		}
		total += size
	}
	return total, nil
}

func (p *ProcType) InstanceIds() (ids []string, err error) {
	revs, err := p.Dir.Snapshot.getdir(p.Dir.prefix("instances"))
	if err != nil {
		return
	}
	for _, rev := range revs {
		// TODO: remove this once all old instances are cleaned up
		if len(rev) != 7 {
			continue
		}

		iids, e := p.Dir.Snapshot.getdir(p.Dir.prefix("instances", rev))
		if e != nil {
			return nil, e
		}
		ids = append(ids, iids...)
	}
	return
}

func (p *ProcType) GetFailedInstances() (ins []*Instance, err error) {
	ids, err := p.Dir.Snapshot.getdir(p.failedInstancesPath())
	if err != nil {
		return
	}
	return p.getInstances(ids)
}

func (p *ProcType) GetInstances() (ins []*Instance, err error) {
	ids, err := p.InstanceIds()
	if err != nil {
		return
	}
	return p.getInstances(ids)
}

func (p *ProcType) getInstances(ids []string) (ins []*Instance, err error) {
	results, err := getSnapshotables(ids, func(idstr string) (snapshotable, error) {
		id, err := strconv.ParseInt(idstr, 10, 64)
		if err != nil {
			return nil, err
		}
		return GetInstance(p.Dir.Snapshot, id)
	})
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		ins = append(ins, r.(*Instance))
	}
	return
}

// TODO consider moving to (*App).GetProcType(name)

// GetProcType fetches a ProcType from the coordinator
func GetProcType(s Snapshot, app *App, name string) (p *ProcType, err error) {
	path := app.Dir.prefix(procsPath, string(name))

	port, err := s.getFile(path+"/port", new(intCodec))
	if err != nil {
		return
	}
	p = NewProcType(app, name, s)
	p.Port = port.Value.(int)

	return
}

func (p *ProcType) String() string {
	return fmt.Sprintf("ProcType<%s:%s>", p.App.Name, p.Name)
}

func (p *ProcType) Inspect() string {
	return fmt.Sprintf("%#v", p)
}
