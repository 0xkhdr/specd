// Package pack implements specd's declarative scaffold packs: named
// bundles of files (with template variables) that `specd init`/`new` can
// apply to seed a spec directory. Pack manifests are pure data — parsing
// explicitly rejects any hook/exec/command/script field and any file path
// that could escape the project root — so applying a pack can only ever
// write files, never run code. Built-in packs are embedded at build time
// via go:embed and validated through the same ParsePack path as
// user-supplied manifests.
package pack
