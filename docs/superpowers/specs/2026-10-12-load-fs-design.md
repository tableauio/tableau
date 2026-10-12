# Load `fs.FS` integration (Design C)

**Status:** implemented  
**Branch:** `feat/load-path-relative-to-dir`  
**Date:** 2026-10-12

## Goal

Replace `ReadFunc` / `WithReadFunc` with `fs.FS` so Path resolution, PatchPaths
glob, and file reads share one filesystem abstraction.

## Decision: C — FS-only

- Remove `ReadFunc` / `GetReadFunc` / `WithReadFunc`.
- Remove `LoadFunc` / `GetLoadFunc` / `WithLoadFunc` (IO customization is FS-only).
- Add `FS` / `WithFileSystem` / `GetFS`.
- Default FS is `hostFS` (wraps `os.Open` / `os.ReadFile` / `filepath.Glob`):
  cwd-relative names; absolute paths and `..` allowed (unlike `os.DirFS`).
- Load always uses `fs.ReadFile` / `fs.Glob` on `GetFS()` — no nil-FS branch.

## Path / PatchPaths

Unchanged empty / relative / absolute resolution against the load `dir`, then
IO via `GetFS()`. Zero glob matches is an error.

`PatchDirs` remain host directories (`xfs.Exists` + join).

## Migration

- `WithReadFunc` callers → `WithFileSystem` (or rely on default `hostFS`).

## Non-goals

- Input formats (Excel/CSV/XML/YAML) stay on `os` / `filepath.Glob`.
