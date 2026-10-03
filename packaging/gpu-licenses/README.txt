Third-party notices for the Voxy/Nopal GPU worker

This directory covers all external packages resolved by gpu/Cargo.lock,
including Linux Vulkan and macOS Metal dependencies. Some entries are
build-only or belong to other targets; inclusion is intentionally broad.
index.json identifies each exact package, SPDX declaration, repository,
revision when available, notice origin and SHA256. Crate-provided notices
are copied verbatim. Missing crate files are retrieved from pinned
upstream revisions; r-efi includes its README/AUTHORS and common Apache2.
This collection does not establish a project-wide license for Voxy.

Regenerate: python3 scripts/generate-gpu-licenses.py
Requires Cargo, Python3 and network access for registry/pinned sources.
