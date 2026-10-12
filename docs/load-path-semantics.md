# Load Path / PatchPaths semantics (draft)

## Problem

`MessagerOptions.Path` historically overrode the load `dir` entirely: whatever
string you put in `Path` was opened as-is. That works for a single tree, but
breaks when the **same** options are reused with **different** dirs — e.g.
tableau CheckCompatibility loading `old/` then `new/` with one
`MessagerOptions` map (binpb / scatter / ModeOnlyPatch).

Absolute `Path` values cannot be shared across trees. Callers then invent
workarounds (`LoadFunc` wrappers that rewrite paths per dir).

## Proposal

Resolve `Path` and each `PatchPaths` entry against the load `dir`:

| value | resolution |
| --- | --- |
| empty | default: `dir/<MessagerName>.<ext>` (unchanged) |
| relative | `filepath.Join(dir, path)` |
| absolute (`filepath.IsAbs`) | use as-is |

`PatchDirs` stay as before (each entry is a directory; files are
`Join(patchDir, Name.ext)`). Only path *files* get the relative-to-dir rule.

## Migration

- Prefer **relative** paths when options are reused across dirs
  (`"scatter/Foo.binpb"`, `"../patch/Foo.json"`).
- Keep **absolute** paths for one-off overrides.
- Call sites that previously passed CWD-relative paths that were *not*
  relative to `dir` (e.g. `Path: "../testdata/..."` while `dir` was also under
  testdata) must switch to dir-relative or absolute paths.

## API surface

- `GetPath` / `GetPatchPaths` still return the **configured** strings.
- Resolution is internal (`resolveConfigPath`) at load time.

## Status

Implemented on branch `feat/load-path-relative-to-dir`. Breaking change for
callers that relied on CWD-relative `Path`/`PatchPaths` ignoring `dir`.
