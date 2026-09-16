package main

import(
 "os"
 "net"
 "encoding/json"
 "fmt"
 "strings"
 "os/exec"
 "path/filepath"
 "testing"
 "time"
)
func TestLockReleasedAfterForcedExit(t *testing.T){
 if dir:=os.Getenv("VOXY_LOCK_HELPER");dir!=""{a:=&app{state:dir};unlock,e:=a.lock("control");if e!=nil{os.Exit(2)};defer unlock();os.WriteFile(a.path("ready"),nil,0600);time.Sleep(time.Minute);return}
 a:=&app{state:t.TempDir()};cmd:=exec.Command(os.Args[0],"-test.run=^TestLockReleasedAfterForcedExit$");cmd.Env=append(os.Environ(),"VOXY_LOCK_HELPER="+a.state);if e:=cmd.Start();e!=nil{t.Fatal(e)};defer cmd.Process.Kill()
 deadline:=time.Now().Add(10*time.Second);for{if _,e:=os.Stat(a.path("ready"));e==nil{break};if time.Now().After(deadline){t.Fatal("helper never acquired lock")};time.Sleep(20*time.Millisecond)}
 if unlock,e:=a.lock("control");e==nil{unlock();t.Fatal("concurrent writer accepted")}
 if e:=cmd.Process.Kill();e!=nil{t.Fatal(e)};_ =cmd.Wait()
 unlock,e:=a.lock("control");if e!=nil{t.Fatal("abandoned kernel lock not released:",e)};unlock()
}
func checkpointFixture(t *testing.T,a *app)checkpoint{
 t.Helper();c:=checkpoint{ID:"20260916T120000.000000000Z",Hashes:map[string]string{}}
 dir:=a.path(filepath.Join("backups",c.ID));if e:=os.MkdirAll(dir,0700);e!=nil{t.Fatal(e)}
 for _,name:=range backupFiles{os.WriteFile(filepath.Join(dir,name),[]byte("snapshot-"+name),0600);c.Hashes[name],_=hashFile(filepath.Join(dir,name));os.WriteFile(a.path(name),[]byte("original-"+name),0600)}
 if e:=durableJSON(filepath.Join(dir,"manifest.json"),c);e!=nil{t.Fatal(e)};return c
}
func TestRestoreRejectsCorruptionBeforeChangingDisk(t *testing.T){
 a:=&app{state:t.TempDir()};c:=checkpointFixture(t,a)
 os.WriteFile(a.path(filepath.Join("backups",c.ID,"kernel")),[]byte("corrupt"),0600)
 if e:=a.restore(c.ID);e==nil{t.Fatal("corruption accepted")};b,_:=os.ReadFile(a.path("disk.qcow2"));if string(b)!="original-disk.qcow2"{t.Fatal("original disk modified")}
}
func TestRestoreResumesAfterInterruptedReplacement(t *testing.T){
 a:=&app{state:t.TempDir()};c:=checkpointFixture(t,a);j:=restoreJournal{c.ID,"before-restore-20260916T130000.000000000Z"}
 os.Mkdir(a.path(j.Previous),0700)
 // Simulate power loss after saving the original disk and restoring only that file.
 os.Rename(a.path("disk.qcow2"),a.path(filepath.Join(j.Previous,"disk.qcow2")))
 copyFile(a.path(filepath.Join("backups",c.ID,"disk.qcow2")),a.path("disk.qcow2"))
 durableJSON(a.path("restore-pending.json"),j)
 if e:=a.applyRestore();e!=nil{t.Fatal(e)}
 for _,name:=range backupFiles{b,_:=os.ReadFile(a.path(name));if string(b)!="snapshot-"+name{t.Fatal("mixed restored generation",name)};b,_=os.ReadFile(a.path(filepath.Join(j.Previous,name)));if string(b)!="original-"+name{t.Fatal("original not preserved",name)}}
 if e:=a.applyRestore();e!=nil{t.Fatal("idempotent retry:",e)}
}
func TestIncompleteBackupsNeverListed(t *testing.T){
 a:=&app{state:t.TempDir()};c:=checkpointFixture(t,a)
 os.Mkdir(a.path("backups/.partial-20260916T140000.000000000Z"),0700)
 os.Mkdir(a.path("backups/20260916T150000.000000000Z"),0700)
 all,e:=a.checkpoints();if e!=nil||len(all)!=1||all[0].ID!=c.ID{t.Fatal(all,e)}
 for _,id:=range []string{"../escape","","latest","20260916T150000.000000000Z/../../x"}{if _,e:=a.validateCheckpoint(id);e==nil{t.Fatal("invalid id accepted",id)}}
}

// Exercise the actual automatic-backup decision with a QMP peer and real process
// identity. No disk copy or SSH operation may occur when write counters match.
func TestAutomaticBackupSkipsUnchangedDisk(t *testing.T) {
 dir,e:=os.MkdirTemp("","vxy-");if e!=nil{t.Fatal(e)};defer os.RemoveAll(dir)
 a:=&app{state:dir};c:=checkpointFixture(t,a)
 exe,created,_,e:=processIdentity(uint32(os.Getpid()));if e!=nil{t.Fatal(e)}
 durableJSON(a.path("process.json"),runState{PID:uint32(os.Getpid()),Created:created,Executable:exe})
 c.QEMUCreated=created;c.Writes=42;durableJSON(a.path(filepath.Join("backups",c.ID,"manifest.json")),c)
 listener,e:=net.Listen("unix",a.path("qmp.sock"));if e!=nil{t.Fatal(e)};defer listener.Close()
 done:=make(chan error,1)
 go func(){conn,e:=listener.Accept();if e!=nil{done<-e;return};defer conn.Close();enc:=json.NewEncoder(conn);dec:=json.NewDecoder(conn);enc.Encode(map[string]any{"QMP":map[string]any{}})
  for i:=0;i<2;i++ {var req struct{Execute,ID string};if e=dec.Decode(&req);e!=nil{done<-e;return};var result any=map[string]any{};if i==0&&req.Execute!="qmp_capabilities"{done<-fmt.Errorf("unexpected command %s",req.Execute);return};if i==1 {if req.Execute!="query-blockstats"{done<-fmt.Errorf("unexpected command %s",req.Execute);return};result=[]any{map[string]any{"device":"rootdisk","stats":map[string]any{"wr_operations":42,"unmap_operations":0}}}};enc.Encode(map[string]any{"return":result,"id":req.ID})};done<-nil
 }()
 if e=a.backup(true);e!=nil{t.Fatal(e)};if e=<-done;e!=nil{t.Fatal(e)}
 all,e:=a.checkpoints();if e!=nil||len(all)!=1{t.Fatal("unexpected checkpoint created",all,e)}
 var status backupStatus;b,_:=os.ReadFile(a.path("backup-status.json"));json.Unmarshal(b,&status);if !strings.HasPrefix(status.Message,"Sin escrituras"){t.Fatal(status)}
}
