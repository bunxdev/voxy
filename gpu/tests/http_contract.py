#!/usr/bin/env python3
"""Integration checks against a real hardware GPU; prints no bearer token."""
import json, os, pathlib, signal, socket, subprocess, sys, tempfile, time, threading

binary = str(pathlib.Path(sys.argv[1]).resolve())

def wait_ready(proc, directory):
    for _ in range(600):
        if (directory/'server.ready').exists():
            return int((directory/'server.port').read_text()), (directory/'token').read_text().strip()
        if proc.poll() is not None:
            raise AssertionError('server exited before ready: '+proc.stdout.read().decode())
        time.sleep(.05)
    raise AssertionError('server readiness timeout')

def request(port, token, method='GET', path='/v1/info', extra='', body=b'', authenticated=True):
    auth = f'Authorization: Bearer {token}\r\n' if authenticated else ''
    data=(f'{method} {path} HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\n{auth}{extra}Content-Length: {len(body)}\r\n\r\n').encode()+body
    return raw(port,data)

def raw(port,data):
    with socket.create_connection(('127.0.0.1',port),timeout=15) as sock:
        sock.sendall(data); sock.shutdown(socket.SHUT_WR); output=b''
        while True:
            part=sock.recv(65536)
            if not part:break
            output+=part
            if b"\r\n\r\n" in output:
                header, payload=output.split(b"\r\n\r\n",1)
                size=int(next(line.split(b":",1)[1] for line in header.split(b"\r\n") if line.lower().startswith(b"content-length:")))
                if len(payload)>=size:break
    header, body=output.split(b'\r\n\r\n',1)
    return int(header.split()[1]),json.loads(body)

with tempfile.TemporaryDirectory(prefix='voxy gpu tests ') as directory:
    directory=pathlib.Path(directory)
    watch=subprocess.Popen(['sleep','300'])
    proc=subprocess.Popen([binary,'serve','--state-dir',str(directory/'state'),'--watch-pid',str(watch.pid)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        state=directory/'state'; port,token=wait_ready(proc,state)
        assert (state.stat().st_mode&0o777)==0o700
        for name in ('token','server.pid','server.port','server.ready','server.lock'):
            assert ((state/name).stat().st_mode&0o777)==0o600
        assert int((state/'server.pid').read_text())==proc.pid
        assert request(port,token)[0]==200
        assert request(port,token,authenticated=False)[0]==401
        assert request(port,'f'*64)[0]==401
        assert request(port,token,extra='Origin: http://localhost\r\n')[0]==403
        assert request(port,token,extra='Transfer-Encoding: chunked\r\n')[0]==400
        assert request(port,token,extra='Content-Length: 0\r\n')[0]==400
        assert request(port,token,extra=f'Authorization: Bearer {token}\r\n')[0]==400
        assert request(port,token,path='/missing')[0]==404
        assert request(port,token,method='POST',path='/v1/compute',body=b'{}')[0]==415
        assert request(port,token,method='POST',path='/v1/compute',extra='Content-Type: application/json\r\n',body=b'{')[0]==400
        oversized=f'POST /v1/compute HTTP/1.1\r\nHost: 127.0.0.1\r\nAuthorization: Bearer {token}\r\nContent-Length: 16777217\r\n\r\n'.encode()
        assert raw(port,oversized)[0]==413
        signed_length=f'POST /v1/self-test HTTP/1.1\r\nHost: 127.0.0.1\r\nAuthorization: Bearer {token}\r\nContent-Length: +0\r\n\r\n'.encode()
        assert raw(port,signed_length)[0]==400
        truncated=f'POST /v1/compute HTTP/1.1\r\nHost: 127.0.0.1\r\nAuthorization: Bearer {token}\r\nContent-Type: application/json\r\nContent-Length: 10\r\n\r\n{{'.encode()
        assert raw(port,truncated)[0]==400
        status,result=request(port,token,method='POST',path='/v1/self-test')
        assert status==200 and result['passed'],result
        assert result['adapter']['device_type'] in ('DiscreteGpu','IntegratedGpu')
        # Freeze the owned subprocess, not a shader/driver, to test cancellation safely.
        active_result=[]
        active=threading.Thread(target=lambda:active_result.append(request(port,token,method='POST',path='/v1/self-test')))
        active.start(); worker_pid=None
        for _ in range(200):
            table=subprocess.check_output(['ps','-axo','pid=,ppid='],text=True)
            children=[int(fields[0]) for line in table.splitlines() if len(fields:=line.split())==2 and int(fields[1])==proc.pid]
            if children:
                worker_pid=children[0]
                try:os.kill(worker_pid,signal.SIGSTOP);break
                except ProcessLookupError:worker_pid=None
            time.sleep(.005)
        assert worker_pid is not None,'could not observe owned worker'
        assert request(port,token)[0]==200
        assert request(port,token,method='POST',path='/v1/self-test')[0]==429
        # A client holding a partial header must not prevent shutdown or cleanup.
        slow=socket.create_connection(('127.0.0.1',port)); slow.sendall(b'GET /')
        assert request(port,token,method='POST',path='/v1/shutdown')[0]==200
        proc.wait(timeout=3);slow.close();active.join(timeout=3);assert proc.returncode==0
        assert active_result and active_result[0][0]==400
        try:os.kill(worker_pid,0)
        except ProcessLookupError:pass
        else:raise AssertionError("GPU subprocess was not reaped")
        worker_pid=None
        assert not list(state.iterdir())
        # Restart, then stop the watched process: daemon exits, deletes its own state.
        proc=subprocess.Popen([binary,'serve','--state-dir',str(state),'--watch-pid',str(watch.pid)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        wait_ready(proc,state);watch.terminate();watch.wait();proc.wait(timeout=3)
        assert proc.returncode==0 and not list(state.iterdir())
        # SIGTERM also cleans up without touching the watched process.
        watch=subprocess.Popen(['sleep','300'])
        proc=subprocess.Popen([binary,'serve','--state-dir',str(state),'--watch-pid',str(watch.pid)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        wait_ready(proc,state);proc.terminate();proc.wait(timeout=3)
        assert proc.returncode==0 and watch.poll() is None and not list(state.iterdir())
        # A launcher may exit and its original process group may be reaped.
        # The helper must keep its actual PID while surviving that group cleanup.
        launcher=subprocess.Popen(['/bin/bash','-c','nohup "$1" serve --state-dir "$2" --watch-pid "$3" > "$4" 2>&1 < /dev/null & echo "$!"', 'gpu-launcher',binary,str(state),str(watch.pid),str(directory/'detached.log')],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
        launched, errors=launcher.communicate(timeout=5)
        assert launcher.returncode==0,errors
        detached_pid=int(launched.strip())
        for _ in range(600):
            if (state/'server.ready').exists():break
            time.sleep(.05)
        assert (state/'server.ready').exists()
        assert os.getsid(detached_pid)==detached_pid
        try:os.killpg(launcher.pid,signal.SIGTERM)
        except ProcessLookupError:pass
        time.sleep(.1);os.kill(detached_pid,0)
        port=int((state/'server.port').read_text());token=(state/'token').read_text().strip()
        assert request(port,token)[0]==200
        assert request(port,token,method='POST',path='/v1/shutdown')[0]==200
        for _ in range(60):
            if not list(state.iterdir()):break
            time.sleep(.05)
        assert not list(state.iterdir())
        detached_pid=None
        print(json.dumps({'passed':True,'checks':['hardware self-test','authentication','Origin denial','framing ambiguity','JSON errors','body limit','truncated body','private state','shutdown with slow client','watcher cleanup','SIGTERM cleanup','active worker cancellation','GPU serialization','survives launcher process-group cleanup']}))
    finally:
        if locals().get("detached_pid"):
            try:os.kill(detached_pid,signal.SIGTERM)
            except ProcessLookupError:pass
        if locals().get("worker_pid"):
            try:os.kill(worker_pid,signal.SIGKILL)
            except ProcessLookupError:pass
        for child in (proc,watch):
            if child.poll() is None:child.kill();child.wait()
