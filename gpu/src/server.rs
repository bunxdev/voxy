use serde_json::{Value, json};
use std::{
    fs::{self, OpenOptions},
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    os::unix::fs::{DirBuilderExt, MetadataExt, OpenOptionsExt, PermissionsExt},
    path::{Path, PathBuf},
    process::{Command, Stdio},
    sync::{
        Arc, Mutex,
        atomic::{AtomicBool, Ordering},
    },
    thread,
    time::{Duration, Instant},
};
static SIGNAL_STOP: AtomicBool = AtomicBool::new(false);
extern "C" fn signal_stop(_: libc::c_int) {
    SIGNAL_STOP.store(true, Ordering::SeqCst);
}
const OUTPUT_LIMIT: usize = 128 * 1024 * 1024;
const DEADLINE: Duration = Duration::from_secs(60);
struct State {
    dir: PathBuf,
    files: Vec<&'static str>,
}
impl State {
    fn create(dir: &Path) -> Result<Self, String> {
        if !dir.exists() {
            fs::DirBuilder::new()
                .recursive(true)
                .mode(0o700)
                .create(dir)
                .map_err(|e| e.to_string())?;
        }
        let metadata = fs::symlink_metadata(dir).map_err(|e| e.to_string())?;
        if !metadata.is_dir()
            || metadata.file_type().is_symlink()
            || metadata.uid() != unsafe { libc::geteuid() }
        {
            return Err("state directory must be a real directory owned by current user".into());
        }
        fs::set_permissions(dir, fs::Permissions::from_mode(0o700)).map_err(|e| e.to_string())?;
        let mut result = Self {
            dir: dir.into(),
            files: Vec::new(),
        };
        result.write("server.lock", &std::process::id().to_string())?;
        Ok(result)
    }
    fn write(&mut self, name: &'static str, content: &str) -> Result<(), String> {
        let mut f = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(self.dir.join(name))
            .map_err(|e| format!("cannot create {name}: {e}"))?;
        self.files.push(name);
        f.write_all(content.as_bytes()).map_err(|e| e.to_string())?;
        if !content.ends_with('\n') {
            f.write_all(b"\n").map_err(|e| e.to_string())?;
        }
        f.sync_all().map_err(|e| e.to_string())
    }
}
impl Drop for State {
    fn drop(&mut self) {
        for name in self.files.iter().rev() {
            let _ = fs::remove_file(self.dir.join(name));
        }
    }
}
fn alive(pid: i32) -> bool {
    unsafe {
        libc::kill(pid, 0) == 0
            || std::io::Error::last_os_error().raw_os_error() == Some(libc::EPERM)
    }
}
fn read_limited(mut reader: impl Read, limit: usize, exceeded: Arc<AtomicBool>) -> Vec<u8> {
    let mut data = Vec::new();
    let mut buf = [0; 8192];
    loop {
        match reader.read(&mut buf) {
            Ok(0) => break,
            Ok(n) => {
                if data.len() + n > limit {
                    exceeded.store(true, Ordering::SeqCst);
                    break;
                }
                data.extend_from_slice(&buf[..n]);
            }
            Err(_) => break,
        }
    }
    data
}
fn worker(command: &str, input: Vec<u8>, stop: &AtomicBool) -> Result<Value, String> {
    let exe = std::env::current_exe().map_err(|e| e.to_string())?;
    let mut child = Command::new(exe)
        .arg(command)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| e.to_string())?;
    let stdin = child.stdin.take().unwrap();
    let out = child.stdout.take().unwrap();
    let err = child.stderr.take().unwrap();
    let exceeded = Arc::new(AtomicBool::new(false));
    let e1 = exceeded.clone();
    let e2 = exceeded.clone();
    let writer = thread::spawn(move || {
        let mut stdin = stdin;
        let _ = stdin.write_all(&input);
    });
    let stdout = thread::spawn(move || read_limited(out, OUTPUT_LIMIT, e1));
    let stderr = thread::spawn(move || read_limited(err, 64 * 1024, e2));
    let start = Instant::now();
    let failure = loop {
        match child.try_wait() {
            Ok(Some(_)) => break None,
            Err(e) => break Some(e.to_string()),
            _ => {}
        }
        if stop.load(Ordering::SeqCst) || SIGNAL_STOP.load(Ordering::SeqCst) {
            break Some("worker cancelled because server is shutting down".into());
        }
        if exceeded.load(Ordering::SeqCst) {
            break Some("worker output exceeded limit".into());
        }
        if start.elapsed() > DEADLINE {
            break Some("GPU worker exceeded 60 second deadline".into());
        }
        thread::sleep(Duration::from_millis(20));
    };
    if failure.is_some() {
        let _ = child.kill();
    }
    let status = child.wait().map_err(|e| e.to_string())?;
    let _ = writer.join();
    let output = stdout.join().unwrap_or_default();
    let _ = stderr.join();
    if let Some(e) = failure {
        return Err(e);
    }
    if exceeded.load(Ordering::SeqCst) {
        return Err("worker output exceeded limit".into());
    }
    let value: Value =
        serde_json::from_slice(&output).map_err(|_| "worker returned invalid JSON")?;
    if !status.success() {
        return Err(value
            .get("error")
            .and_then(Value::as_str)
            .unwrap_or("GPU worker failed")
            .to_string());
    }
    Ok(value)
}
struct Request {
    method: String,
    path: String,
    body: Vec<u8>,
}
fn read_bytes(
    stream: &mut TcpStream,
    output: &mut Vec<u8>,
    count: usize,
    deadline: Instant,
) -> Result<(), (u16, String)> {
    let mut buf = [0; 8192];
    while output.len() < count {
        if SIGNAL_STOP.load(Ordering::SeqCst) {
            return Err((503, "server shutting down".into()));
        }
        if Instant::now() > deadline {
            return Err((408, "request timeout".into()));
        }
        let want = (count - output.len()).min(buf.len());
        match stream.read(&mut buf[..want]) {
            Ok(0) => return Err((400, "incomplete request".into())),
            Ok(n) => output.extend_from_slice(&buf[..n]),
            Err(e)
                if matches!(
                    e.kind(),
                    std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                ) => {}
            Err(_) => return Err((400, "request read failed".into())),
        }
    }
    Ok(())
}
fn equal_token(a: &str, b: &str) -> bool {
    if a.len() != b.len() {
        return false;
    }
    a.bytes().zip(b.bytes()).fold(0u8, |x, (a, b)| x | (a ^ b)) == 0
}
fn parse(stream: &mut TcpStream, token: &str) -> Result<Request, (u16, String)> {
    stream
        .set_read_timeout(Some(Duration::from_millis(250)))
        .map_err(|_| (500, "socket timeout unavailable".into()))?;
    let deadline = Instant::now() + Duration::from_secs(10);
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        if header.len() >= 16384 {
            return Err((431, "headers too large".into()));
        }
        let count = header.len() + 1;
        read_bytes(stream, &mut header, count, deadline)?;
    }
    let text = std::str::from_utf8(&header).map_err(|_| (400, "invalid headers".into()))?;
    let mut lines = text.split("\r\n");
    let mut request = lines.next().unwrap_or("").split_whitespace();
    let method = request.next().unwrap_or("").to_string();
    let path = request.next().unwrap_or("").to_string();
    if request.next() != Some("HTTP/1.1") || request.next().is_some() {
        return Err((400, "HTTP/1.1 required".into()));
    }
    let mut auth = None;
    let mut size = None;
    let mut content_type = None;
    let mut host = None;
    for line in lines.filter(|s| !s.is_empty()) {
        let (key, value) = line
            .split_once(':')
            .ok_or((400, "malformed header".into()))?;
        if key.is_empty()
            || !key.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'-')
            || value.bytes().any(|b| b < 32 && b != b'\t')
        {
            return Err((400, "invalid header characters".into()));
        }
        let value = value.trim();
        match key.to_ascii_lowercase().as_str() {
            "origin" => {
                return Err((
                    403,
                    "browser Origin is not accepted by the GPU worker".into(),
                ));
            }
            "authorization" => {
                if auth.replace(value).is_some() {
                    return Err((400, "duplicate authorization".into()));
                }
            }
            "content-length" => {
                if size.is_some() {
                    return Err((400, "duplicate content length".into()));
                }
                if value.is_empty() || !value.bytes().all(|b| b.is_ascii_digit()) {
                    return Err((400, "invalid content length".into()));
                }
                size = Some(
                    value
                        .parse::<usize>()
                        .map_err(|_| (400, "invalid content length".into()))?,
                );
            }
            "transfer-encoding" => return Err((400, "transfer encoding is not supported".into())),
            "content-type" => {
                if content_type.replace(value).is_some() {
                    return Err((400, "duplicate content type".into()));
                }
            }
            "host" => {
                if host.replace(value).is_some() {
                    return Err((400, "duplicate host".into()));
                }
            }
            _ => {}
        }
    }
    if !host.is_some_and(|h| {
        h == "localhost"
            || h.starts_with("localhost:")
            || h == "127.0.0.1"
            || h.starts_with("127.0.0.1:")
    }) {
        return Err((403, "loopback Host required".into()));
    }
    if !auth
        .and_then(|a| a.strip_prefix("Bearer "))
        .is_some_and(|a| equal_token(a, token))
    {
        return Err((401, "authorization required".into()));
    }
    let length = size.unwrap_or(0);
    if length > crate::MAX_BODY {
        return Err((413, "request exceeds 16 MiB".into()));
    }
    if method == "POST"
        && path == "/v1/compute"
        && !content_type.is_some_and(|s| s.split(';').next() == Some("application/json"))
    {
        return Err((415, "application/json required".into()));
    }
    let mut body = Vec::new();
    read_bytes(stream, &mut body, length, deadline)?;
    Ok(Request { method, path, body })
}
fn respond(stream: &mut TcpStream, status: u16, value: Value) {
    let body = value.to_string();
    let reason = match status {
        200 => "OK",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        408 => "Request Timeout",
        413 => "Content Too Large",
        415 => "Unsupported Media Type",
        429 => "Too Many Requests",
        431 => "Request Header Fields Too Large",
        _ => "Internal Server Error",
    };
    let _ = stream.set_write_timeout(Some(Duration::from_secs(5)));
    let _ = write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\nX-Content-Type-Options: nosniff\r\n\r\n{body}",
        body.len()
    );
    let _ = stream.flush();
    // Send the complete response before closing; macOS may discard it with RST
    // if a rejected request body remains unread. Drain only a bounded remainder.
    let _ = stream.shutdown(std::net::Shutdown::Write);
    let _ = stream.set_read_timeout(Some(Duration::from_millis(100)));
    let deadline = Instant::now() + Duration::from_millis(200);
    let mut remaining = 65536usize;
    let mut scratch = [0u8; 8192];
    while remaining > 0 && Instant::now() < deadline {
        let amount = remaining.min(scratch.len());
        match stream.read(&mut scratch[..amount]) {
            Ok(0) => break,
            Ok(n) => remaining -= n,
            Err(_) => break,
        }
    }
}
fn handle(
    mut stream: TcpStream,
    token: Arc<String>,
    info: Arc<Value>,
    busy: Arc<Mutex<()>>,
    stop: Arc<AtomicBool>,
) {
    let request = match parse(&mut stream, &token) {
        Ok(r) => r,
        Err((code, e)) => {
            respond(&mut stream, code, json!({"error":e}));
            return;
        }
    };
    match (request.method.as_str(), request.path.as_str()) {
        ("GET", "/v1/info") => respond(&mut stream, 200, (*info).clone()),
        ("POST", "/v1/shutdown") => {
            respond(&mut stream, 200, json!({"stopped":true}));
            stop.store(true, Ordering::SeqCst);
            SIGNAL_STOP.store(true, Ordering::SeqCst);
        }
        ("POST", "/v1/compute") | ("POST", "/v1/self-test") => {
            let _guard = match busy.try_lock() {
                Ok(g) => g,
                Err(_) => {
                    respond(&mut stream, 429, json!({"error":"GPU worker is busy"}));
                    return;
                }
            };
            let command = if request.path == "/v1/compute" {
                "compute"
            } else {
                "self-test"
            };
            match worker(command, request.body, &stop) {
                Ok(result) => respond(&mut stream, 200, result),
                Err(e) => respond(&mut stream, 400, json!({"error":e})),
            }
        }
        _ => respond(&mut stream, 404, json!({"error":"unknown endpoint"})),
    }
}
pub fn serve(args: &[String]) -> Result<Value, String> {
    // Keep the real PID while detaching from the launcher's terminal/process group.
    // A launcher (including an exec tool) may otherwise reap its background group.
    if unsafe { libc::getsid(0) } != unsafe { libc::getpid() }
        && unsafe { libc::setsid() } == -1
        && unsafe { libc::getsid(0) } != unsafe { libc::getpid() }
    {
        return Err(format!(
            "cannot create GPU server session: {}",
            std::io::Error::last_os_error()
        ));
    }
    unsafe {
        libc::signal(
            libc::SIGTERM,
            signal_stop as *const () as libc::sighandler_t,
        );
        libc::signal(libc::SIGINT, signal_stop as *const () as libc::sighandler_t);
    }
    let mut dir = None;
    let mut watch = None;
    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--state-dir" => {
                i += 1;
                dir = Some(PathBuf::from(args.get(i).ok_or("missing state directory")?));
            }
            "--watch-pid" => {
                i += 1;
                watch = Some(
                    args.get(i)
                        .ok_or("missing watch PID")?
                        .parse::<i32>()
                        .map_err(|_| "invalid watch PID")?,
                );
            }
            _ => return Err("unknown serve argument".into()),
        }
        i += 1;
    }
    let pid = watch.ok_or("--watch-pid required")?;
    if pid <= 1 || !alive(pid) {
        return Err("watch PID must identify a running process greater than 1".into());
    }
    let dir = dir.ok_or("--state-dir required")?;
    let mut state = State::create(&dir)?;
    let stop = Arc::new(AtomicBool::new(false));
    let info = Arc::new(worker("probe", Vec::new(), &stop)?);
    if info["available"] != true {
        return Err(info["reason"]
            .as_str()
            .unwrap_or("GPU unavailable")
            .to_string());
    }
    let mut entropy = [0; 32];
    getrandom::fill(&mut entropy).map_err(|e| e.to_string())?;
    let token = Arc::new(
        entropy
            .iter()
            .map(|b| format!("{b:02x}"))
            .collect::<String>(),
    );
    let listener = TcpListener::bind(("127.0.0.1", 0)).map_err(|e| e.to_string())?;
    listener.set_nonblocking(true).map_err(|e| e.to_string())?;
    state.write("token", &token)?;
    state.write(
        "server.port",
        &listener
            .local_addr()
            .map_err(|e| e.to_string())?
            .port()
            .to_string(),
    )?;
    state.write("server.pid", &std::process::id().to_string())?;
    state.write("server.ready", "ready\n")?;
    let busy = Arc::new(Mutex::new(()));
    let mut threads = Vec::<thread::JoinHandle<()>>::new();
    while !stop.load(Ordering::SeqCst) && !SIGNAL_STOP.load(Ordering::SeqCst) && alive(pid) {
        threads.retain(|t| !t.is_finished());
        match listener.accept() {
            Ok((mut stream, _)) => {
                if threads.len() >= 8 {
                    respond(&mut stream, 429, json!({"error":"too many connections"}));
                } else {
                    let token = token.clone();
                    let info = info.clone();
                    let busy = busy.clone();
                    let stop = stop.clone();
                    threads.push(thread::spawn(move || {
                        handle(stream, token, info, busy, stop)
                    }));
                }
            }
            Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(30))
            }
            Err(e) => {
                stop.store(true, Ordering::SeqCst);
                for t in threads {
                    let _ = t.join();
                }
                return Err(e.to_string());
            }
        }
    }
    stop.store(true, Ordering::SeqCst);
    SIGNAL_STOP.store(true, Ordering::SeqCst);
    drop(listener);
    for t in threads {
        let _ = t.join();
    }
    drop(state);
    Ok(json!({"stopped":true}))
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn token_comparison() {
        assert!(equal_token("abc", "abc"));
        assert!(!equal_token("abc", "abd"));
        assert!(!equal_token("abc", "abcd"));
    }
}
