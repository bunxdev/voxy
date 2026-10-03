use serde::Deserialize;
use serde_json::{Value, json};
use std::{collections::HashSet, time::Instant};
use wgpu::util::DeviceExt;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Request {
    pub shader: String,
    pub entry_point: String,
    pub workgroups: [u32; 3],
    pub buffers: Vec<InputBuffer>,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct InputBuffer {
    pub binding: u32,
    pub data: Vec<f32>,
    pub readback: bool,
}

fn hardware(info: &wgpu::AdapterInfo) -> bool {
    let name = info.name.to_ascii_lowercase();
    matches!(
        info.device_type,
        wgpu::DeviceType::DiscreteGpu | wgpu::DeviceType::IntegratedGpu
    ) && !["llvmpipe", "lavapipe", "software", "swiftshader"]
        .iter()
        .any(|s| name.contains(s))
        && if cfg!(target_os = "linux") {
            info.backend == wgpu::Backend::Vulkan && info.vendor == 0x10de
        } else if cfg!(target_os = "macos") {
            info.backend == wgpu::Backend::Metal
        } else {
            false
        }
}
fn adapter() -> Result<wgpu::Adapter, String> {
    if !cfg!(any(target_os = "linux", target_os = "macos")) {
        return Err("GPU compute is supported only on Linux NVIDIA/Vulkan and macOS Metal".into());
    }
    let backend = if cfg!(target_os = "macos") {
        wgpu::Backends::METAL
    } else {
        wgpu::Backends::VULKAN
    };
    let instance = wgpu::Instance::new(wgpu::InstanceDescriptor {
        backends: backend,
        ..wgpu::InstanceDescriptor::new_without_display_handle()
    });
    pollster::block_on(instance.enumerate_adapters(backend))
        .into_iter()
        .find(|a| hardware(&a.get_info()))
        .ok_or_else(|| "no supported hardware GPU found; CPU/software fallback is disabled".into())
}
fn info(a: &wgpu::Adapter) -> Value {
    let i = a.get_info();
    json!({"available":true,"name":i.name,"backend":format!("{:?}",i.backend),"vendor":i.vendor,"device":i.device,"device_type":format!("{:?}",i.device_type),"driver":i.driver,"driver_info":i.driver_info})
}
pub fn probe() -> Value {
    std::panic::catch_unwind(|| match adapter() {
        Ok(a) => info(&a),
        Err(e) => json!({"available":false,"name":null,"backend":null,"vendor":null,"device_type":null,"reason":e}),
    }).unwrap_or_else(|_| json!({"available":false,"name":null,"backend":null,"vendor":null,"device_type":null,"reason":"GPU initialization failed"}))
}
fn validate(r: &Request) -> Result<(), String> {
    if r.shader.is_empty() || r.shader.len() > 65536 {
        return Err("shader must contain 1..65536 bytes".into());
    }
    if r.entry_point.is_empty()
        || r.entry_point.len() > 128
        || !r
            .entry_point
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_')
    {
        return Err("invalid entry_point".into());
    }
    if r.workgroups.iter().any(|&n| n == 0 || n > 65535)
        || r.workgroups.iter().map(|&n| n as u64).product::<u64>() > 1_048_576
    {
        return Err("workgroups exceed limits (1..65535 each, 1048576 total)".into());
    }
    if r.buffers.is_empty() || r.buffers.len() > 8 {
        return Err("1..8 buffers required".into());
    }
    let mut bindings = HashSet::new();
    let mut total = 0usize;
    for b in &r.buffers {
        if b.binding >= 8 || !bindings.insert(b.binding) {
            return Err("bindings must be distinct integers in 0..7".into());
        }
        if b.data.is_empty() || b.data.iter().any(|n| !n.is_finite()) {
            return Err("buffers require nonempty finite f32 data".into());
        }
        total = total
            .checked_add(b.data.len() * 4)
            .ok_or("buffer size overflow")?;
    }
    if total > 32 * 1024 * 1024 {
        return Err("buffer data exceeds 32 MiB".into());
    }
    Ok(())
}
pub fn compute(r: Request) -> Result<Value, String> {
    validate(&r)?;
    pollster::block_on(compute_async(r))
}
async fn check(guards: [wgpu::ErrorScopeGuard; 3]) -> Result<(), String> {
    let mut errors = Vec::new();
    for guard in guards.into_iter().rev() {
        if let Some(e) = guard.pop().await {
            errors.push(e.to_string());
        }
    }
    if errors.is_empty() {
        Ok(())
    } else {
        Err(format!("GPU error: {}", errors.join("; ")))
    }
}
fn scopes(device: &wgpu::Device) -> [wgpu::ErrorScopeGuard; 3] {
    [
        device.push_error_scope(wgpu::ErrorFilter::Internal),
        device.push_error_scope(wgpu::ErrorFilter::OutOfMemory),
        device.push_error_scope(wgpu::ErrorFilter::Validation),
    ]
}
async fn compute_async(r: Request) -> Result<Value, String> {
    let start = Instant::now();
    let a = adapter()?;
    let adapter_info = info(&a);
    let (device, queue) = a
        .request_device(&wgpu::DeviceDescriptor {
            label: Some("voxy-gpu"),
            ..Default::default()
        })
        .await
        .map_err(|e| format!("request device: {e}"))?;
    let guards = scopes(&device);
    let module = device.create_shader_module(wgpu::ShaderModuleDescriptor {
        label: Some("trusted compute shader"),
        source: wgpu::ShaderSource::Wgsl(r.shader.into()),
    });
    check(guards).await?;
    let guards = scopes(&device);
    let entries: Vec<_> = r
        .buffers
        .iter()
        .map(|b| wgpu::BindGroupLayoutEntry {
            binding: b.binding,
            visibility: wgpu::ShaderStages::COMPUTE,
            ty: wgpu::BindingType::Buffer {
                ty: wgpu::BufferBindingType::Storage { read_only: false },
                has_dynamic_offset: false,
                min_binding_size: None,
            },
            count: None,
        })
        .collect();
    let layout = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
        label: None,
        entries: &entries,
    });
    let pipeline_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
        label: None,
        bind_group_layouts: &[Some(&layout)],
        immediate_size: 0,
    });
    let pipeline = device.create_compute_pipeline(&wgpu::ComputePipelineDescriptor {
        label: None,
        layout: Some(&pipeline_layout),
        module: &module,
        entry_point: Some(&r.entry_point),
        compilation_options: Default::default(),
        cache: None,
    });
    check(guards).await?;
    let guards = scopes(&device);
    let buffers: Vec<_> = r
        .buffers
        .iter()
        .map(|b| {
            device.create_buffer_init(&wgpu::util::BufferInitDescriptor {
                label: None,
                contents: bytemuck::cast_slice(&b.data),
                usage: wgpu::BufferUsages::STORAGE | wgpu::BufferUsages::COPY_SRC,
            })
        })
        .collect();
    let entries: Vec<_> = r
        .buffers
        .iter()
        .zip(&buffers)
        .map(|(b, gpu)| wgpu::BindGroupEntry {
            binding: b.binding,
            resource: gpu.as_entire_binding(),
        })
        .collect();
    let group = device.create_bind_group(&wgpu::BindGroupDescriptor {
        label: None,
        layout: &layout,
        entries: &entries,
    });
    let mut encoder =
        device.create_command_encoder(&wgpu::CommandEncoderDescriptor { label: None });
    {
        let mut pass = encoder.begin_compute_pass(&wgpu::ComputePassDescriptor {
            label: None,
            timestamp_writes: None,
        });
        pass.set_pipeline(&pipeline);
        pass.set_bind_group(0, &group, &[]);
        pass.dispatch_workgroups(r.workgroups[0], r.workgroups[1], r.workgroups[2]);
    }
    let mut outputs = Vec::new();
    for (input, source) in r.buffers.iter().zip(&buffers).filter(|(b, _)| b.readback) {
        let size = (input.data.len() * 4) as u64;
        let staging = device.create_buffer(&wgpu::BufferDescriptor {
            label: None,
            size,
            usage: wgpu::BufferUsages::COPY_DST | wgpu::BufferUsages::MAP_READ,
            mapped_at_creation: false,
        });
        encoder.copy_buffer_to_buffer(source, 0, &staging, 0, size);
        outputs.push((input.binding, staging));
    }
    let submission = queue.submit([encoder.finish()]);
    device
        .poll(wgpu::PollType::Wait {
            submission_index: Some(submission),
            timeout: Some(std::time::Duration::from_secs(50)),
        })
        .map_err(|e| format!("GPU wait: {e}"))?;
    check(guards).await?;
    let mut result = Vec::new();
    for (binding, staging) in outputs {
        let (tx, rx) = std::sync::mpsc::channel();
        staging.slice(..).map_async(wgpu::MapMode::Read, move |r| {
            let _ = tx.send(r);
        });
        device
            .poll(wgpu::PollType::Wait {
                submission_index: None,
                timeout: Some(std::time::Duration::from_secs(5)),
            })
            .map_err(|e| format!("GPU map wait: {e}"))?;
        rx.recv_timeout(std::time::Duration::from_secs(5))
            .map_err(|e| e.to_string())?
            .map_err(|e| e.to_string())?;
        let view = staging
            .slice(..)
            .get_mapped_range()
            .map_err(|e| e.to_string())?;
        let data: Vec<f32> = bytemuck::cast_slice(&view).to_vec();
        if data.iter().any(|x| !x.is_finite()) {
            return Err("GPU result contains nonfinite f32 values".into());
        }
        result.push(json!({"binding":binding,"data":data}));
        drop(view);
        staging.unmap();
    }
    Ok(
        json!({"adapter":adapter_info,"buffers":result,"elapsed_ms":start.elapsed().as_secs_f64()*1000.0}),
    )
}
pub fn self_test() -> Result<Value, String> {
    let start = Instant::now();
    let n = 4096usize;
    let input: Vec<f32> = (0..n).map(|i| (i as f32 - 1024.0) * 0.25).collect();
    let vector=compute(Request { shader:"@group(0) @binding(0) var<storage, read_write> data: array<f32>; @compute @workgroup_size(64) fn main(@builtin(global_invocation_id) id: vec3<u32>) { if (id.x < arrayLength(&data)) { data[id.x] = data[id.x] * 3.0 + 7.0; } }".into(), entry_point:"main".into(), workgroups:[64,1,1], buffers:vec![InputBuffer{binding:0,data:input.clone(),readback:true}] })?;
    let got = vector["buffers"][0]["data"]
        .as_array()
        .ok_or("missing vector result")?;
    if got.len() != n
        || got
            .iter()
            .zip(&input)
            .any(|(v, x)| v.as_f64() != Some((x * 3.0 + 7.0) as f64))
    {
        return Err("GPU vector verification failed".into());
    }
    let dim = 32usize;
    let left: Vec<f32> = (0..dim * dim)
        .map(|i| ((i * 7 % 19) as f32 - 9.0) / 8.0)
        .collect();
    let right: Vec<f32> = (0..dim * dim)
        .map(|i| ((i * 11 % 23) as f32 - 11.0) / 8.0)
        .collect();
    let matrix=compute(Request { shader:"@group(0) @binding(0) var<storage,read_write> a:array<f32>; @group(0) @binding(1) var<storage,read_write> b:array<f32>; @group(0) @binding(2) var<storage,read_write> c:array<f32>; @compute @workgroup_size(8,8) fn main(@builtin(global_invocation_id) id:vec3<u32>) { if(id.x>=32u || id.y>=32u){return;} var sum=0.0; for(var k=0u;k<32u;k=k+1u){sum=sum+a[id.y*32u+k]*b[k*32u+id.x];} c[id.y*32u+id.x]=sum; }".into(), entry_point:"main".into(), workgroups:[4,4,1], buffers:vec![InputBuffer{binding:0,data:left.clone(),readback:false},InputBuffer{binding:1,data:right.clone(),readback:false},InputBuffer{binding:2,data:vec![0.0;dim*dim],readback:true}] })?;
    let got = matrix["buffers"][0]["data"]
        .as_array()
        .ok_or("missing matrix result")?;
    if got.len() != dim * dim {
        return Err("GPU matrix length mismatch".into());
    }
    for y in 0..dim {
        for x in 0..dim {
            let expected: f32 = (0..dim)
                .map(|k| left[y * dim + k] * right[k * dim + x])
                .sum();
            if (got[y * dim + x].as_f64().ok_or("invalid matrix number")? - expected as f64).abs()
                > 1e-4
            {
                return Err("GPU matrix verification failed".into());
            }
        }
    }
    Ok(
        json!({"passed":true,"adapter":matrix["adapter"],"vector":{"elements":n,"verified":true},"matmul":{"shape":[dim,dim],"verified":true},"elapsed_ms":start.elapsed().as_secs_f64()*1000.0,"cpu_fallback":false}),
    )
}
#[cfg(test)]
mod tests {
    use super::*;
    fn request() -> Request {
        Request {
            shader: "test".into(),
            entry_point: "main".into(),
            workgroups: [1, 1, 1],
            buffers: vec![InputBuffer {
                binding: 0,
                data: vec![1.0],
                readback: true,
            }],
        }
    }
    #[test]
    fn resource_limits() {
        let mut r = request();
        assert!(validate(&r).is_ok());
        r.workgroups = [65535, 65535, 65535];
        assert!(validate(&r).is_err());
        r = request();
        r.buffers[0].data[0] = f32::NAN;
        assert!(validate(&r).is_err());
        r = request();
        r.buffers.push(InputBuffer {
            binding: 0,
            data: vec![0.0],
            readback: false,
        });
        assert!(validate(&r).is_err());
    }
}
