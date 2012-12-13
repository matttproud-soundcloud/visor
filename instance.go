// Copyright (c) 2012, SoundCloud Ltd.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/visor

package visor

import (
	"code.google.com/p/goprotobuf/proto"
	"encoding/json"
	"fmt"
	"github.com/soundcloud/doozer"
	"github.com/soundcloud/visor/generated"
	"path"
	"strconv"
	"strings"
)

const (
	pathInstanceRoot             = "instances"
	pathInstanceEntityDefinition = "entity-definition"
)

const claimsPath = "claims"
const failedPath = "failed"
const startPath = "start"
const statusPath = "status"
const stopPath = "stop"

const (
	InsStatusPending  InsStatus = "pending"
	InsStatusClaimed            = "claimed"
	InsStatusRunning            = "running"
	InsStatusStopping           = "stopping"

	InsStatusFailed = "failed"
	InsStatusExited = "exited"
)

// Instance represents application instances.
type Instance struct {
	Dir          dir
	Id           int64
	AppName      string
	RevisionName string
	ProcessName  string
	Ip           string
	Port         int
	Host         string
	Status       InsStatus
	state        *generated.Instance
}

// GetInstance returns an Instance from the given id
func GetInstance(s Snapshot, id int64) (ins *Instance, err error) {
	p := instancePath(id)
	status := InsStatusPending

	var (
		ip   string
		port int
		host string
	)

	f, err := s.getFile(p+"/start", new(listCodec))
	if IsErrNoEnt(err) {
	} else if err != nil {
		return
	} else {
		fields := f.Value.([]string)

		if len(fields) > 0 { // IP
			ip = fields[0]
		}
		if len(fields) > 1 { // Port
			status = InsStatusRunning
			port, err = strconv.Atoi(fields[1])
			if err != nil {
				panic("invalid port number: " + fields[1])
			}
		}
		if len(fields) > 2 { // Hostname
			host = fields[2]
		}
	}

	statusStr, _, err := s.get(p + "/status")
	if IsErrNoEnt(err) {
		err = nil
	} else if err == nil {
		status = InsStatus(statusStr)
	} else {
		return
	}

	if status == InsStatusRunning {
		_, _, err := s.get(p + "/stop")
		if err == nil {
			status = InsStatusStopping
		} else if !IsErrNoEnt(err) {
			return nil, err
		}
	}

	f, err = s.getFile(p+"/object", new(listCodec))
	if err != nil {
		return
	}
	fields := f.Value.([]string)

	dir := dir{s, instancePath(id)}
	state := &generated.Instance{}

	if marshaled, _, err := dir.getBytes(pathRevisionEntityDefinition); err == nil {
		if err = json.Unmarshal(marshaled, state); err == nil {
			ins = &Instance{
				Id:           id,
				AppName:      fields[0],
				RevisionName: fields[1],
				ProcessName:  fields[2],
				Status:       status,
				Ip:           ip,
				Port:         port,
				Host:         host,
				Dir:          dir,
				state:        state,
			}
		}
	}
	return
}

func getInstanceIds(s Snapshot, app, rev, pty string) (ids []int64, err error) {
	p := ptyInstancesPath(app, rev, pty)
	exists, _, err := s.conn.Exists(p)
	if err != nil || !exists {
		return
	}

	dir, err := s.getdir(p)
	if err != nil {
		return
	}
	ids = []int64{}
	for _, f := range dir {
		id, e := strconv.ParseInt(f, 10, 64)
		if e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return
}

func RegisterInstance(app string, rev string, pty string, s Snapshot) (ins *Instance, err error) {
	//
	//   instances/
	//       6868/
	// +         object = <app> <rev> <proc>
	// +         start  =
	//
	//   apps/<app>/procs/<proc>/instances/<rev>
	// +     6868 = 2012-07-19 16:41 UTC
	//
	id, err := Getuid(s)
	if err != nil {
		return
	}

	state := &generated.Instance{
		State:           generated.Instance_PENDING.Enum(),
		ApplicationName: proto.String(app),
		RevisionName:    proto.String(rev),
		ProcessName:     proto.String(pty),
	}

	ins = &Instance{
		Id:           id,
		AppName:      app,
		RevisionName: rev,
		ProcessName:  pty,
		Status:       InsStatusPending,
		Dir:          dir{s, instancePath(id)},
		state:        state,
	}

	_, err = createFile(s, ins.Dir.prefix(pathRevisionEntityDefinition), state, new(jsonCodec))
	if err != nil {
		return nil, err
	}

	// XXX: Migrated
	_, err = createFile(s, ins.Dir.prefix("object"), ins.objectArray(), new(listCodec))
	if err != nil {
		return nil, err
	}
	_, err = s.set(ins.ptyInstancesPath(), timestamp())
	if err != nil {
		return
	}

	// XXX: Migratd
	s1, err := createFile(s, ins.Dir.prefix(startPath), "", new(stringCodec))
	if err != nil {
		return nil, err
	}
	ins = ins.FastForward(s1.Snapshot.Rev)
	ins, err = ins.put()

	return
}

func (i *Instance) put() (instance *Instance, err error) {
	if marshaled, err := json.Marshal(i.state); err == nil {
		if rev, err := i.Dir.setBytes(pathInstanceEntityDefinition, marshaled); err == nil {
			instance = i.FastForward(rev)
		}
	}

	return
}

func StopInstance(id int64, s Snapshot) (s1 Snapshot, err error) {
	//
	//   instances/
	//       6868/
	//           ...
	// +         stop =
	//
	// TODO Check that instance is started
	d := dir{s, instancePath(id)}
	rev, err := d.set("stop", "")
	if err != nil {
		return
	}
	s1 = s.FastForward(rev)

	return
}

func instancePath(id int64) string {
	return path.Join(pathInstanceRoot, strconv.FormatInt(id, 10))
}

// FastForward advances the instance in time. It returns
// a new instance of Instance with the supplied revision.
func (i *Instance) FastForward(rev int64) *Instance {
	return i.Dir.Snapshot.fastForward(i, rev).(*Instance)
}

func (i *Instance) createSnapshot(rev int64) snapshotable {
	tmp := *i
	tmp.Dir.Snapshot = Snapshot{rev, i.Dir.Snapshot.conn}
	return &tmp
}

// Claims returns the list of claimers.
func (i *Instance) Claims() (claims []string, err error) {
	rev, err := i.Dir.Snapshot.conn.Rev()
	if err != nil {
		return
	}
	claims, err = i.Dir.Snapshot.conn.Getdir(i.Dir.prefix("claims"), rev)
	if err, ok := err.(*doozer.Error); ok && err.Err == doozer.ErrNoEnt {
		claims = []string{}
		err = nil
	}
	return
}

// Claim locks the instance to the specified host.
func (i *Instance) Claim(host string) (*Instance, error) {
	//
	//   instances/
	//       6868/
	//           claims/
	// +             10.0.0.1 = 2012-07-19 16:22 UTC
	//           object = <app> <rev> <proc>
	// -         start  =
	// +         start  = 10.0.0.1
	//
	val, rev, err := i.Dir.get(startPath)
	if err != nil {
		return nil, err
	}
	if val != "" {
		return nil, ErrInsClaimed
	}
	d := i.Dir.fastForward(rev)

	_, err = d.set(startPath, host)
	if err != nil {
		return i, err
	}

	rev, err = i.claimDir().fastForward(rev).set(host, timestamp())
	if err != nil {
		return i, err
	}
	return i.FastForward(rev), err
}

func (i *Instance) Unregister() (err error) {
	err = i.Dir.Snapshot.del(i.ptyInstancesPath())
	if err != nil {
		if IsErrNoEnt(err) {
			err = nil
		} else {
			return
		}
	}
	err = i.Dir.del("/")
	return
}

// Exited tells the coordinator that the instance has exited.
func (i *Instance) Exited(host string) (i1 *Instance, err error) {
	if err = i.verifyClaimer(host); err != nil {
		return
	}
	i1, err = i.updateStatus(InsStatusExited)
	if err != nil {
		return nil, err
	}
	err = i.Dir.Snapshot.del(i.ptyInstancesPath())

	return
}

func (i *Instance) started(ip string, port int, host string) {
	i.Ip = ip
	i.Port = port
	i.Host = host
	i.Status = InsStatusRunning
}

func (i *Instance) claimed(ip string) {
	i.Ip = ip
	i.Status = InsStatusClaimed
}

func (i *Instance) Started(host string, port int, hostname string) (i1 *Instance, err error) {
	//
	//   instances/
	//       6868/
	//           object = <app> <rev> <proc>
	// -         start  = 10.0.0.1
	// +         start  = 10.0.0.1 24690 localhost
	//
	if i.Status == InsStatusRunning {
		return i, nil
	}
	if err = i.verifyClaimer(host); err != nil {
		return
	}
	i1 = i.FastForward(i.Dir.Snapshot.Rev) // Create a copy
	i1.started(host, port, hostname)

	f, err := createFile(i1.Dir.Snapshot, i1.Dir.prefix(startPath), i1.startArray(), new(listCodec))
	if err != nil {
		return
	}
	i1 = i1.FastForward(f.FileRev)

	return
}

func (i *Instance) updateStatus(s InsStatus) (i1 *Instance, err error) {
	rev, err := i.Dir.set("status", string(s))
	if err != nil {
		return
	}
	i.Status = s

	return i.FastForward(rev), err
}

func (i *Instance) getClaimer() (*string, error) {
	f, err := i.Dir.Snapshot.getFile(i.Dir.prefix(startPath), new(listCodec))

	if IsErrNoEnt(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	fields := f.Value.([]string)

	if len(fields) == 0 {
		return nil, nil
	}
	return &fields[0], nil
}

func (i *Instance) setClaimer(claimer string) (rev int64, err error) {
	return i.Dir.set(startPath, claimer)
}

// Unclaim removes the lock applied by Claim of the Ticket.
func (i *Instance) Unclaim(host string) (i1 *Instance, err error) {
	//
	//   instances/
	//       6868/
	// -         start = 10.0.0.1
	// +         start =
	//
	if err = i.verifyClaimer(host); err != nil {
		return
	}

	rev, err := i.setClaimer("")
	if err != nil {
		return
	}
	i1 = i.FastForward(rev)

	return
}

func (i *Instance) verifyClaimer(host string) error {
	claimer, err := i.getClaimer()
	if err != nil {
		return err
	} else if claimer == nil || *claimer != host {
		return ErrUnauthorized
	}
	return nil
}

func (i *Instance) Failed(host string, reason error) (i1 *Instance, err error) {
	if err = i.verifyClaimer(host); err != nil {
		return
	}
	_, err = i.updateStatus(InsStatusFailed)
	if err != nil {
		return
	}
	s, err := i.Dir.Snapshot.set(i.ptyFailedPath(), timestamp()+" "+reason.Error())
	if err != nil {
		return
	}
	err = i.Dir.Snapshot.del(i.ptyInstancesPath())
	if err != nil {
		return
	}
	i1 = i.FastForward(s.Rev)

	return
}

func WatchInstanceStart(s Snapshot, listener chan *Instance, errors chan error) {
	// instances/*/start =
	rev := s.Rev

	for {
		ev, err := s.conn.Wait(path.Join(pathInstanceRoot, "*", startPath), rev+1)
		rev = ev.Rev

		if !ev.IsSet() || string(ev.Body) != "" {
			continue
		}
		idstr := strings.Split(ev.Path, "/")[2]

		id, err := strconv.ParseInt(idstr, 0, 64)
		if err != nil {
			panic(err)
		}
		ins, err := GetInstance(s.FastForward(rev), id)
		if err != nil {
			panic(err)
		}
		listener <- ins
	}
}

func (i *Instance) WaitStop() (i1 *Instance, err error) {
	p := path.Join(pathInstanceRoot, strconv.FormatInt(i.Id, 10), stopPath)

	ev, err := i.Dir.Snapshot.conn.Wait(p, i.Dir.Snapshot.Rev+1)
	if err != nil {
		return
	}
	i1 = i.FastForward(ev.Rev)

	return
}

func (i *Instance) WaitStatus() (i1 *Instance, err error) {
	p := path.Join(pathInstanceRoot, strconv.FormatInt(i.Id, 10), statusPath)
	ev, err := i.Dir.Snapshot.conn.Wait(p, i.Dir.Snapshot.Rev+1)
	if err != nil {
		return
	}
	i1 = i.FastForward(ev.Rev)
	i1.Status = InsStatus(string(ev.Body))

	return
}

func (i *Instance) WaitClaimed() (i1 *Instance, err error) {
	return i.waitStartPathStatus(InsStatusClaimed)
}

func (i *Instance) WaitStarted() (i1 *Instance, err error) {
	return i.waitStartPathStatus(InsStatusRunning)
}

func (i *Instance) waitStartPathStatus(s InsStatus) (i1 *Instance, err error) {
	for {
		i, err = i.waitStartPath()
		if err != nil {
			return i, err
		}
		if i.Status == s {
			break
		}
	}
	return i, nil
}

func (i *Instance) waitStartPath() (i1 *Instance, err error) {
	p := path.Join(pathInstanceRoot, strconv.FormatInt(i.Id, 10), startPath)

	ev, err := i.Dir.Snapshot.conn.Wait(p, i.Dir.Snapshot.Rev+1)
	if err != nil {
		return
	}
	i1 = i.FastForward(ev.Rev)
	parts, err := new(listCodec).Decode(ev.Body)
	if err != nil {
		return
	}
	fields := parts.([]string)
	if len(fields) >= 3 {
		ip, host := fields[0], fields[2]
		port, err := strconv.Atoi(fields[1])
		if err != nil {
			return i, err
		}
		i1.started(ip, port, host)
	} else if len(fields) > 0 {
		i1.claimed(fields[0])
	} else {
		// TODO
	}
	return
}

func ptyInstancesPath(app, rev, pty string) string {
	return path.Join(appsPath, app, procsPath, pty, pathInstanceRoot, rev)
}

func (i *Instance) idString() string {
	return fmt.Sprintf("%d", i.Id)
}

func (i *Instance) RefString() string {
	return fmt.Sprintf("%s:%s@%s", i.AppName, i.ProcessName, i.RevisionName)
}

func (i *Instance) ServiceName() string {
	return fmt.Sprintf("%s-%s", i.AppName, i.ProcessName)
}

func (i *Instance) ptyFailedPath() string {
	return path.Join(appsPath, i.AppName, procsPath, i.ProcessName, failedPath, i.idString())
}

func (i *Instance) ptyInstancesPath() string {
	return path.Join(appsPath, i.AppName, procsPath, i.ProcessName, pathInstanceRoot, i.RevisionName, i.idString())
}

func (i *Instance) claimPath(host string) string {
	return i.Dir.prefix("claims", host)
}

func (i *Instance) claimDir() *dir {
	return &dir{i.Dir.Snapshot, i.Dir.prefix(claimsPath)}
}

func (i *Instance) Fields() string {
	return fmt.Sprintf("%d %s %s %s %s %d", i.Id, i.AppName, i.RevisionName, i.ProcessName, i.Ip, i.Port)
}

func (i *Instance) objectArray() []string {
	return []string{i.AppName, i.RevisionName, i.ProcessName}
}

func (i *Instance) startArray() []string {
	return []string{i.Ip, i.portString(), i.Host}
}

func (i *Instance) portString() string {
	return fmt.Sprintf("%d", i.Port)
}

// String returns the Go-syntax representation of Instance.
func (i *Instance) String() string {
	return fmt.Sprintf("Instance{id=%d, app=%s, rev=%s, proc=%s, addr=%s:%d}", i.Id, i.AppName, i.RevisionName, i.ProcessName, i.Ip, i.Port)
}

// IdString returns a string of the format "INSTANCE[id]"
func (i *Instance) IdString() string {
	return fmt.Sprintf("INSTANCE[%d]", i.Id)
}

func (i *Instance) Failures() int32 {
	return *i.state.Failures
}

func (i *Instance) ResetFailures() (instance *Instance, err error) {
	i.state.Failures = proto.Int32(0)

	return i.put()
}

func (i *Instance) IncrementFailures() (instance *Instance, err error) {
	i.state.Failures = proto.Int32((*i.state.Failures) + 1)

	return i.put()
}
