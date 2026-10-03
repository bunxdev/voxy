# Voxy GPU worker

Hardware-only compute for Voxy/Nopal. Linux requires NVIDIA through Vulkan; macOS uses Metal on an integrated or discrete GPU. CPU adapters, llvmpipe/lavapipe and other software adapters are rejected. Windows is currently unsupported. A headless host works: no display surface or browser WebGPU support is needed.

This bridge runs **trusted WGSL shaders on the host GPU**. It does not expose CUDA, a GPU PCI device, or transparent PyTorch acceleration inside the Debian VM. The host retains its GPU and can share it with other applications; jobs compete for the same resources.

## Build and CLI

Rust 1.87 or later, with Cargo.lock committed:

```sh
cargo build --release --locked --manifest-path gpu/Cargo.toml
voxy-gpu probe
voxy-gpu probe --require
voxy-gpu self-test
voxy-gpu compute < request.json
voxy-gpu serve --state-dir '/private/path/gpu' --watch-pid 12345
```

`probe` always exits zero and returns `{available,name,backend,vendor,device_type,reason?}`. `--require` exits nonzero when unavailable. The adapter includes its physical device identifier and driver information when available. `self-test` dispatches a 4096-element vector transform and a 32×32 matrix multiplication, reads GPU results and checks them against reference arithmetic; those CPU comparisons are verification, never a fallback implementation.

Example compute input:

```json
{"shader":"@group(0) @binding(0) var<storage,read_write> data:array<f32>; @compute @workgroup_size(1) fn main(@builtin(global_invocation_id) id:vec3<u32>) {data[id.x]=data[id.x]*2.0;}","entry_point":"main","workgroups":[3,1,1],"buffers":[{"binding":0,"data":[1,2,3],"readback":true}]}
```

Returns `{adapter,buffers:[{binding,data}],elapsed_ms}`. `elapsed_ms` includes adapter/device initialization, compilation, dispatch and readback; it is not a GPU-only timer. Bindings are storage `read_write` in group 0. Buffers marked `readback:false` are omitted from the result.

Limits: 1–8 buffers with distinct bindings 0–7, 32 MiB of total f32 input data, 64 KiB of WGSL, entry-point names up to 128 ASCII letters/digits/underscores, 1–65535 workgroups per dimension and at most 1,048,576 workgroups total. Input and output floats must be finite. Device limits and shader validation also apply. These limits do not make arbitrary shaders safe: a shader can loop indefinitely or monopolize the driver.

## Local HTTP protocol

`serve` listens only on `127.0.0.1` using an ephemeral port. It writes `token` (64 random hex characters), `server.port`, `server.pid`, `server.ready` and an ownership lock in the state directory. The directory is mode 0700 and files are 0600; the PID is the actual server process. Existing files are not overwritten. Do not share the token or log it. Local processes running as the same user can read it, so this is not a security boundary against that user.

Every endpoint requires `Authorization: Bearer TOKEN`:

- `GET /v1/info`: adapter probe result.
- `POST /v1/compute`: JSON compute request (`Content-Type: application/json`).
- `POST /v1/self-test`: hardware self-test.
- `POST /v1/shutdown`: completes its JSON response, cancels active work and stops.

All requests require an HTTP/1.1 loopback Host. Browser `Origin` headers are rejected; CORS is not enabled. The guest HTTP proxy must authenticate independently, validate its own browser origin, and remove Origin only after that validation. Use the authenticated SSH tunnel provided by the launcher to reach the host service; never bind it to a public interface.

Bodies are limited to 16 MiB, headers to 16 KiB, and total request reading to 10 seconds. Chunked bodies, transfer encodings, duplicate framing/authentication headers and malformed headers are rejected. Connections close after each response. A maximum of eight connections is serviced concurrently, with one GPU operation at a time; another GPU request receives 429.

Each HTTP GPU operation uses a subprocess with a 60-second deadline and bounded captured stdout/stderr (128 MiB/64 KiB). Shutdown, SIGINT, SIGTERM or loss of the watched process cancels the subprocess and removes only state files created by this server. SIGKILL, host crashes or kernel/driver hangs may leave stale files; inspect their PID before removing them. Killing a process does **not** guarantee immediate cancellation of work already submitted to a GPU driver. Direct CLI `compute`, `self-test`, and launcher `gpu run` are trusted local commands; they do not have the HTTP supervisor deadline. Do not run untrusted shaders or use an intentionally infinite GPU shader to test timeout handling.

## Verification

```sh
cargo test --locked --manifest-path gpu/Cargo.toml
python3 gpu/tests/http_contract.py /path/to/voxy-gpu
bash gpu/tests/launcher_contract.sh /path/to/voxy-gpu
```

The HTTP suite requires supported physical hardware. It checks actual compute, authentication, Origin rejection, framing and size errors, permissions, shutdown, signal handling and watched-process cleanup. Cancellation is tested by pausing only the owned worker process with SIGSTOP, then confirming shutdown kills and reaps it; no infinite GPU shader is executed. The launcher suite checks Bash state-file reads, paths with spaces, exact process identity and stale-state cleanup without stopping unrelated processes. On Linux, an additional negative test can restrict Vulkan to the installed software ICD for **one process only**: `VK_DRIVER_FILES=/path/to/lvp_icd.json voxy-gpu probe --require` must fail, and `compute` must not calculate on the CPU.

For independent runtime evidence, correlate the worker PID, NVIDIA device activity and successful readback; utilization alone is insufficient when other applications share the GPU. The worker never changes host drivers, modules, boot parameters, PCI binding, or other GPU clients.
