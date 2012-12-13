package visor

import (
	"math/rand"
)

var ticketId int64 = 10
var appNames = []string{"cat", "dog", "bird", "wolf", "bear", "lion", "tiger"}
var revNames = []string{"master", "slave", "e7fa81", "a91748", "f7ea91", "dev", "stable"}

func genApp(s Snapshot) (app *App) {
	name := randItem(appNames)
	app = NewApp(name, "git://"+name+".git", "my-stack", s)
	app, err := app.Register()
	if err != nil {
		panic(err)
	}
	return
}

func genRevision(app *App) (rev *Revision) {
	name := randItem(revNames)
	rev = NewRevision(app, name, app.Dir.Snapshot)
	rev, err := rev.Propose()
	if err != nil {
		panic(err)
	}
	rev, err = rev.Accept("/path/to/artifact")
	if err != nil {
		panic(err)
	}
	return
}

func genProctype(app *App, name string) (pty *ProcType) {
	pty = NewProcType(app, name, app.Dir.Snapshot)
	pty, err := pty.Register()
	if err != nil {
		panic(err)
	}
	return
}

//func Instance(pty *visor.ProcType, rev *visor.Revision, s visor.Snapshot) (ins *visor.Instance) {
//	if pty == nil {
//		pty = ProcType(nil, s, randItem(ptyNames))
//	}
//	if rev == nil {
//		rev = Revision(nil, s)
//	}
//	addr := fmt.Sprintf("127.0.0.1:%d", 8000+rand.Int63n(1000))
//	ins, err := visor.NewInstance(string(pty.Name), rev.Ref, rev.App.Name, addr, s)
//	if err != nil {
//		panic(err)
//	}
//	return
//}

func randItem(items []string) string {
	return items[rand.Int63n(int64(len(items)))]
}
