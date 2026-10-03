mod gpu;
#[cfg(unix)]
mod server;
use serde_json::{Value, json};
use std::io::{self, Read};

const MAX_BODY: usize = 16 * 1024 * 1024;
fn run() -> Result<Value, String> {
    let args: Vec<String> = std::env::args().collect();
    match args.get(1).map(String::as_str) {
        Some("probe") => Ok(gpu::probe()),
        Some("compute") => {
            let mut bytes = Vec::new();
            io::stdin()
                .take((MAX_BODY + 1) as u64)
                .read_to_end(&mut bytes)
                .map_err(|e| e.to_string())?;
            if bytes.len() > MAX_BODY {
                return Err("request exceeds 16 MiB".into());
            }
            let request =
                serde_json::from_slice(&bytes).map_err(|e| format!("invalid JSON: {e}"))?;
            gpu::compute(request)
        }
        Some("self-test") => gpu::self_test(),
        #[cfg(unix)]
        Some("serve") => server::serve(&args[2..]),
        _ => Err(
            "usage: voxy-gpu probe|compute|self-test|serve --state-dir DIR --watch-pid PID".into(),
        ),
    }
}
fn main() {
    let result = std::panic::catch_unwind(run)
        .unwrap_or_else(|_| Err("GPU worker failed unexpectedly".into()));
    match result {
        Ok(v) => {
            println!("{v}");
            if std::env::args().nth(1).as_deref() == Some("probe")
                && std::env::args().any(|a| a == "--require")
                && v["available"] != true
            {
                std::process::exit(1);
            }
        }
        Err(e) => {
            println!("{}", json!({"error":e}));
            std::process::exit(1);
        }
    }
}
